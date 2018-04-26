#!/bin/sh

#Copy binaries to bin
cp mozartctl/mozartctl mozart-server/mozart-server /usr/local/bin

#Add path to system wide profile
echo 'export PATH=$PATH:/usr/local/bin' >> /etc/profile

#Create directories
mkdir -p /etc/mozart/ssl/
mkdir -p /var/lib/mozart/

#Create mozart-server systemd file
cat > /etc/systemd/system/mozart-server.service <<EOF
[Unit]
Description=Mozart Server
Documentation=https://github.com/zbblanton/mozart_alpha

[Service]
ExecStart=/usr/local/bin/mozart-server
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
sudo systemctl enable mozart-server