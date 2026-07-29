package cloud

import (
	"context"

	"github.com/vmlens/vmlens/backend/internal/model"
)

// Provider is the control-plane boundary between VMLens observability and a
// real cloud platform such as OpenStack, AWS, or a private virtualization API.
//
// The first production-safe step is read-only context. Write actions
// (create/update/delete firewall rules or routes) should be implemented by a
// separate action interface with audit, preview, validation, and rollback.
type Provider interface {
	Name() string
	Status(ctx context.Context) (model.CloudProviderStatus, error)
	ListHosts(ctx context.Context, scope model.OwnershipScope) ([]model.CloudHost, error)
	ListPublicIPs(ctx context.Context, scope model.OwnershipScope) ([]model.PublicIP, error)
	ListFirewallRules(ctx context.Context, scope model.OwnershipScope) ([]model.FirewallRule, error)
	ListRoutes(ctx context.Context, scope model.OwnershipScope) ([]model.NetworkRoute, error)
	ListNetworkPolicies(ctx context.Context, scope model.OwnershipScope) ([]model.NetworkPolicy, error)
}

type NoopProvider struct{}

func NewNoopProvider() NoopProvider { return NoopProvider{} }

func (NoopProvider) Name() string { return "local-db" }

func (NoopProvider) Status(context.Context) (model.CloudProviderStatus, error) {
	return model.CloudProviderStatus{
		Name:      "local-db",
		Mode:      "read_only_model",
		Connected: true,
		Capabilities: []string{
			"vm_inventory_model",
			"public_ip_model",
			"firewall_rule_model",
			"route_model",
			"network_policy_model",
			"connection_intent_model",
			"observed_connection_model",
		},
	}, nil
}

func (NoopProvider) ListHosts(context.Context, model.OwnershipScope) ([]model.CloudHost, error) {
	return nil, nil
}

func (NoopProvider) ListPublicIPs(context.Context, model.OwnershipScope) ([]model.PublicIP, error) {
	return nil, nil
}

func (NoopProvider) ListFirewallRules(context.Context, model.OwnershipScope) ([]model.FirewallRule, error) {
	return nil, nil
}

func (NoopProvider) ListRoutes(context.Context, model.OwnershipScope) ([]model.NetworkRoute, error) {
	return nil, nil
}

func (NoopProvider) ListNetworkPolicies(context.Context, model.OwnershipScope) ([]model.NetworkPolicy, error) {
	return nil, nil
}
