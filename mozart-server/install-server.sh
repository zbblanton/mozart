#!/bin/sh

CLUSTERNAME=testcluster1

cat > /etc/systemd/system/mozart-server.service <<EOF
[Unit]
Description=Mozart Server
Documentation=https://github.com/zbblanton/mozart_alpha

[Service]
ExecStart=/usr/local/bin/mozart-server \\
  --config=/etc/mozart/$CLUSTERNAME-config.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
sudo systemctl enable mozart-server
sudo systemctl start mozart-server