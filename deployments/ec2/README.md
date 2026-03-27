# EC2 Node Setup

## Instance Specs
- Type: `t4g.nano` (ARM64, 512 MB RAM, 2 vCPU burst)
- OS: Amazon Linux 2023 ARM64
- Region: `us-east-1`

## Services Running
| Container | Port | Purpose |
|-----------|------|---------|
| `autodream-browser-pool` | 9222 (localhost only) | Playwright/Chromium ATS automation |
| `autodream-apply-engine` | 8084 | Asynq worker + emergency stop API |

## Initial Setup
1. Terraform creates the EC2 with `setup.sh` as User Data
2. After instance starts: `ssh ec2-user@<IP> sudo systemctl status autodream`
3. Copy `docker-compose.yml` to `/opt/autodream/docker-compose.yml` on the instance
4. Run `sudo systemctl start autodream`

## Deploying New Images
```bash
./deployments/ec2/deploy.sh <ec2-ip> <image-tag>
```

## Memory Budget (512 MB t4g.nano)
| Service | Limit | Notes |
|---------|-------|-------|
| browser-pool | 350 MB | Chromium needs ~300MB per session |
| apply-engine | 120 MB | Go binary, very lean |
| OS + Docker | ~40 MB | Amazon Linux 2023 minimal |
| **Total** | **~510 MB** | At capacity with 1 concurrent session |

> Upgrade to `t4g.micro` ($6.14/mo, 1 GB) if you need 2+ concurrent browser sessions.
