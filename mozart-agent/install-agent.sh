#!/bin/sh
AGENT_IP=10.0.0.10
SERVER_IP=10.0.0.10
JOIN_KEY=PROVIDEKEYHERE
CA_HASH=PROVIDEHASHHERE

cat > /etc/systemd/system/mozart-agent.service <<EOF
[Unit]
Description=Mozart Agent
Documentation=https://github.com/zbblanton/mozart_alpha

[Service]
ExecStart=/usr/local/bin/mozart-agent \\
  --agent=$AGENT_IP \\
  --server=$SEVER_IP \\
  --key=$JOIN_KEY
  --ca-hash=$JOIN_KEY
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
sudo systemctl enable mozart-agent
sudo systemctl start mozart-agent