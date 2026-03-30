# Habanero Agent

Headless network diagnostic agent for Linux. Runs 24/7 on hypervisors, servers, and edge devices. Reports to the Habanero consultant dashboard.

## Install

```bash
curl -fsSL https://get.habanero.tools/agent | sudo bash
```

Or build from source:

```bash
make build
sudo make install
```

Or Docker:

```bash
docker run -d --net=host cosmicllama/habanero-agent
```

## Configure

Edit `/etc/habanero/agent.yml`. See `configs/agent.example.yml`.

## Features

- Diagnostic chain (L1-L5) on schedule
- Continuous ping/DNS/bandwidth monitoring
- Mesh peer probing (site-to-site latency)
- Fleet API reporting to dashboard
- PagerDuty, ServiceNow, Slack integrations
- SNMP polling for switches/routers/APs
- Packet capture and analysis

## License

Copyright 2026 Cosmic Llama LLC. All rights reserved.
