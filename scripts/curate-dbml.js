// Turn raw db2dbml output into two diagrams: the full schema, and the minimal
// profile VMLens actually needs.
//
// db2dbml emits tables in catalog order with every CHECK constraint inline and
// no grouping, which makes the dbdiagram.io editor noisy and the canvas
// arbitrary. This pass keeps the generated content authoritative and only
// rearranges, annotates, and filters it:
//
//   - a Project block with the legend
//   - tables ordered identity -> observation -> probe -> declared-only
//   - headercolor per evidence source (see contracts/README.md)
//   - TableGroup boxes so the unwritten cloud tables cannot be mistaken for live ones
//   - enum-shaped CHECKs promoted to real DBML Enums, so the allowed values stay
//     visible; range CHECKs (>= 0, port bounds) dropped, they live in the migrations
//
// The minimal profile is a PROPOSAL, not a description of the database. It
// drops what the MINIMAL manifest below lists, with a reason per entry. Nothing
// is removed from Postgres by generating it.
//
// Usage: node curate-dbml.js <raw.dbml> <full.dbml> <minimal.dbml>

const fs = require("fs");

// Evidence source -> colour. Blue is what we know about the fleet, teal is what
// eBPF observed, orange is what the probe proved, grey is schema that exists but
// nothing writes.
const GROUPS = [
  {
    name: "Identity (agent + inventory)",
    color: "#3498db",
    tables: ["agents", "vms", "vm_interfaces"],
  },
  {
    name: "Observed traffic (eBPF)",
    color: "#16a085",
    tables: [
      "network_flows",
      "flow_observations",
      "external_hosts",
      "unknown_internal_hosts",
    ],
  },
  {
    name: "Reachability (active probe)",
    color: "#e67e22",
    tables: ["connection_probes"],
  },
  {
    name: "Declared only - nothing writes these",
    color: "#95a5a6",
    tables: [
      "cloud_public_ips",
      "cloud_firewall_rules",
      "cloud_routes",
      "cloud_network_policies",
      "connection_intents",
      "connection_configurations",
      "connection_change_events",
    ],
  },
];

// What the minimal profile drops, and why. Every entry is a decision someone
// has to be able to argue with, so none of them are silent.
const MINIMAL = {
  tables: {
    cloud_public_ips: "cloud provider integration is a noop; no writer exists",
    cloud_firewall_rules: "cloud provider integration is a noop; no writer exists",
    cloud_routes: "cloud provider integration is a noop; no writer exists",
    cloud_network_policies: "cloud provider integration is a noop; no writer exists",
    connection_intents: "intent modelling not built; no writer exists",
    connection_configurations: "intent modelling not built; no writer exists",
    connection_change_events: "no reader and no writer anywhere",
  },
  columns: {
    network_flows: {
      packets: "adds nothing over bytes for topology or health",
      avg_app_delay_ms:
        "needs 3 kprobes on the hottest TCP syscalls, and the number is wrong for multiplexed protocols",
    },
    flow_observations: {
      packets: "adds nothing over bytes for topology or health",
      avg_app_delay_ms: "same as network_flows.avg_app_delay_ms",
    },
    external_hosts: {
      domain: "enrichment that nothing populates",
      asn: "enrichment that nothing populates",
      country: "enrichment that nothing populates",
      provider: "enrichment that nothing populates",
    },
  },
};

// A CHECK that lists allowed values is an enum in everything but the DDL.
// Naming them by value set keeps one Enum shared across the tables that use it;
// an unnamed set is a hard error so a new enum cannot appear unnoticed.
const ENUM_NAMES = new Map([
  ["tcp|udp|icmp", "protocol"],
  ["tcp|udp|icmp|any", "protocol_or_any"],
  ["ingress|egress", "direction"],
  ["online|stale|offline", "node_status"],
  [
    "internal_same_tenant|internal_cross_tenant|unknown_internal|external_public|external_private|unknown",
    "flow_scope",
  ],
  ["private|public|management|unknown", "exposure"],
  ["allow|deny|reject", "policy_action"],
  ["assigned|available|released|unknown", "public_ip_status"],
  ["private|public|default|peering|vpn|unknown", "route_type"],
  ["intended|validated|inactive|deprecated|blocked", "intent_status"],
  ["unknown|allowed|denied|blocked|missing|stale", "config_state"],
  ["unknown|private|public|management|overexposed|blocked", "security_state"],
]);

const FULL_NOTE = `  Note: '''
    VMLens tracks network relationships between VMs: who talked to whom, and how
    well that path performed. This is the FULL schema, generated from
    backend/internal/db/migrations by make schema-dbml. Do not hand-edit.

    Colour legend, one per evidence source:
      blue   identity   - what we know the fleet is (agent registration + inventory file)
      teal   observed   - what eBPF actually saw on the NIC
      orange reachable  - what an active probe proved right now
      grey   declared   - tables that exist but no code writes; empty is not "nothing configured"

    Two traps the diagram cannot show:
      network_flows counters are cumulative; flow_observations rows are window deltas.
      network_flows.src_ip is always the observing VM, never the packet header source.
  '''`;

const MINIMAL_NOTE = `  Note: '''
    MINIMAL PROFILE - a proposal, not a description of the database.

    This is the schema VMLens needs to answer its product question: "is VM A
    connected to VM B, and is that path healthy?" It is the full schema minus
    the tables nothing writes and the columns whose metrics cost more than they
    are worth. See contracts/data-schema.md for the reasoning; each removal is
    commented at the point it was made.

    The matching capture profile is 4 eBPF attachments:
      tcx/ingress, tcx/egress    who talks to whom, bytes, SYN, RST
      kprobe/tcp_rcv_established RTT
      kprobe/tcp_retransmit_skb  retransmissions
    plus the userspace connectivity probe.

    Generating this file changes nothing in Postgres. The columns still exist
    until a migration removes them.
  '''`;

const [, , inputPath, fullPath, minimalPath] = process.argv;
if (!inputPath || !fullPath || !minimalPath) {
  console.error("usage: node curate-dbml.js <raw.dbml> <full.dbml> <minimal.dbml>");
  process.exit(2);
}

const raw = fs.readFileSync(inputPath, "utf8");

// Split the file into table blocks and Ref lines. Table bodies contain nested
// braces (Indexes), so match to the closing brace at column zero.
const tableBlocks = new Map();
const tableRe = /^Table "([^"]+)" \{\n([\s\S]*?)^\}$/gm;
let match;
while ((match = tableRe.exec(raw)) !== null) {
  tableBlocks.set(match[1], match[0]);
}

const allRefs = raw
  .split("\n")
  .filter((line) => line.startsWith("Ref "))
  .sort();

if (tableBlocks.size === 0 || allRefs.length === 0) {
  console.error("curate-dbml: input did not look like db2dbml output");
  process.exit(1);
}

// Every table must be accounted for, otherwise a new migration would silently
// drop out of the diagram.
const grouped = GROUPS.flatMap((group) => group.tables);
const ungrouped = [...tableBlocks.keys()].filter((t) => !grouped.includes(t));
if (ungrouped.length > 0) {
  console.error(
    `curate-dbml: table(s) not assigned to a group in curate-dbml.js: ${ungrouped.join(", ")}`,
  );
  process.exit(1);
}
const absent = grouped.filter((t) => !tableBlocks.has(t));
if (absent.length > 0) {
  console.error(
    `curate-dbml: grouped table(s) missing from the schema: ${absent.join(", ")}`,
  );
  process.exit(1);
}

// Retype the column when its CHECK is a value list, then drop every CHECK.
function applyChecks(block, usedEnums) {
  const lines = block.split("\n").map((line) => {
    const check = line.match(/check: `([^`]*)`/);
    if (!check) {
      return line;
    }
    const column = line.match(/^\s*"([^"]+)" (\w+)/);
    const values = check[1].match(/ARRAY\[([^\]]*)\]/);
    if (!column || !values) {
      return line;
    }
    const list = values[1]
      .split(",")
      .map((value) => value.trim().replace(/::text$/, "").replace(/^'|'$/g, ""));
    const name = ENUM_NAMES.get(list.join("|"));
    if (!name) {
      console.error(
        `curate-dbml: unnamed value set for ${column[1]}: ${list.join("|")}\n` +
          "  add it to ENUM_NAMES in scripts/curate-dbml.js",
      );
      process.exit(1);
    }
    usedEnums.set(name, list);
    return line.replace(`"${column[1]}" ${column[2]}`, `"${column[1]}" ${name}`);
  });

  return lines
    .join("\n")
    .replace(/,\s*check: `[^`]*`/g, "")
    .replace(/check: `[^`]*`,\s*/g, "")
    .replace(/check: `[^`]*`/g, "")
    .replace(/\[\s*\]/g, "")
    .replace(/[ \t]+$/gm, "");
}

// Replace a dropped column with the reason it was dropped, and take it out of
// any index that referenced it.
function dropColumns(block, drops, table) {
  if (!drops) {
    return block;
  }
  const names = Object.keys(drops);
  for (const name of names) {
    if (!new RegExp(`^\\s*"${name}" `, "m").test(block)) {
      console.error(
        `curate-dbml: ${table}.${name} is in the MINIMAL manifest but not in the schema`,
      );
      process.exit(1);
    }
  }
  return block
    .split("\n")
    .flatMap((line) => {
      const column = line.match(/^\s*"([^"]+)" /);
      if (column && names.includes(column[1])) {
        return [`  // dropped: ${column[1]} - ${drops[column[1]]}`];
      }
      // An index over a dropped column cannot survive it.
      const index = line.match(/^\s{4}\(?([a-z_, ]+)\)?\s*\[type: btree/);
      if (index && index[1].split(",").some((c) => names.includes(c.trim()))) {
        return [];
      }
      return [line];
    })
    .join("\n");
}

function withHeaderColor(block, name, color) {
  return block.replace(
    `Table "${name}" {`,
    `Table "${name}" [headercolor: ${color}] {`,
  );
}

function render({ name, note, skipTables, dropColumnsBy }) {
  const usedEnums = new Map();
  const groups = GROUPS.map((group) => ({
    ...group,
    tables: group.tables.filter((t) => !skipTables[t]),
  })).filter((group) => group.tables.length > 0);

  const tables = [];
  for (const group of groups) {
    tables.push(`// ${"=".repeat(70)}`);
    tables.push(`// ${group.name}`);
    tables.push(`// ${"=".repeat(70)}`);
    tables.push("");
    for (const table of group.tables) {
      let block = dropColumns(tableBlocks.get(table), dropColumnsBy[table], table);
      block = applyChecks(block, usedEnums);
      tables.push(withHeaderColor(block, table, group.color));
      tables.push("");
    }
  }

  const out = [`Project "${name}" {`, "  database_type: 'PostgreSQL'", note, "}", ""];

  const skipped = Object.entries(skipTables);
  if (skipped.length > 0) {
    out.push(`// ${"=".repeat(70)}`);
    out.push("// Tables left out of this profile");
    out.push(`// ${"=".repeat(70)}`);
    for (const [table, reason] of skipped) {
      out.push(`//   ${table} - ${reason}`);
    }
    out.push("");
  }

  out.push(`// ${"=".repeat(70)}`);
  out.push("// Enums, promoted from CHECK constraints");
  out.push(`// ${"=".repeat(70)}`);
  out.push("");
  for (const [enumName, values] of [...usedEnums].sort()) {
    out.push(`Enum "${enumName}" {`);
    for (const value of values) {
      out.push(`  "${value}"`);
    }
    out.push("}");
    out.push("");
  }

  out.push(...tables);

  out.push(`// ${"=".repeat(70)}`);
  out.push("// Groups");
  out.push(`// ${"=".repeat(70)}`);
  out.push("");
  for (const group of groups) {
    out.push(`TableGroup "${group.name}" {`);
    for (const table of group.tables) {
      out.push(`  ${table}`);
    }
    out.push("}");
    out.push("");
  }

  const refs = allRefs.filter(
    (ref) => !Object.keys(skipTables).some((t) => ref.includes(`"${t}"`)),
  );
  out.push(`// ${"=".repeat(70)}`);
  out.push("// Relations");
  out.push(`// ${"=".repeat(70)}`);
  out.push("");
  out.push(...refs);
  out.push("");

  console.error(
    `curate-dbml: ${name} - ${groups.flatMap((g) => g.tables).length} tables, ` +
      `${refs.length} refs, ${groups.length} groups, ${usedEnums.size} enums`,
  );
  return out.join("\n");
}

fs.writeFileSync(
  fullPath,
  render({
    name: "vmlens",
    note: FULL_NOTE,
    skipTables: {},
    dropColumnsBy: {},
  }),
);

fs.writeFileSync(
  minimalPath,
  render({
    name: "vmlens-minimal",
    note: MINIMAL_NOTE,
    skipTables: MINIMAL.tables,
    dropColumnsBy: MINIMAL.columns,
  }),
);
