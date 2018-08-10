#!/bin/bash

if [ "$EUID" -ne 0 ]
  then echo "Please run this script as root to install correctly."
  exit
fi

if [[ "$OSTYPE" == "linux-gnu" ]]; then
  [[ ":$PATH:" != *":/usr/local/bin:"* ]] && echo 'export PATH=$PATH:/usr/local/bin' >> /etc/profile
  cp mozartctl /usr/local/bin
  chmod 755 /usr/local/bin/mozartctl
elif [[ "$OSTYPE" == "darwin"* ]]; then
  [[ ":$PATH:" != *":/usr/local/bin:"* ]] && echo 'export PATH=$PATH:/usr/local/bin' >> /etc/bashrc
  cp mozartctl /usr/local/bin
  chmod 755 /usr/local/bin/mozartctl
else
  echo "Cannot detect OS."
fi
