#ifndef VMLENS_BPF_HELPERS_H
#define VMLENS_BPF_HELPERS_H

static __always_inline __u64 vmlens_now_ns(void)
{
    return bpf_ktime_get_ns();
}

#endif
