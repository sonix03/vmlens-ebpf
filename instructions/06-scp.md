# SCP VM ke VM

```bash
# testing-a-1
dd if=/dev/zero of=/tmp/vmlens-test.bin bs=1M count=10
scp /tmp/vmlens-test.bin ubuntu@10.20.20.199:/tmp/
sha256sum /tmp/vmlens-test.bin
ssh ubuntu@10.20.20.199 'sha256sum /tmp/vmlens-test.bin'
```

```bash
# Cleanup
rm /tmp/vmlens-test.bin
ssh ubuntu@10.20.20.199 'rm /tmp/vmlens-test.bin'
```
