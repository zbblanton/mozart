#!/bin/sh
#Copy binary to bin
cp mozartctl /usr/local/bin
chmod 755 /usr/local/bin/mozartctl

#Add path to system wide profile
echo 'export PATH=$PATH:/usr/local/bin' >> /etc/profile

#Create directories
#mkdir -p /etc/mozart/ssl/
#mkdir -p /var/lib/mozart/

