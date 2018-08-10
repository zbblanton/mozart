#!/bin/bash

if [[ "$OSTYPE" == "linux-gnu" ]]; then
  if [ "$EUID" -ne 0 ]
    then echo "Please run this script as root to install correctly."
    exit
  fi
  [[ ":$PATH:" != *":/usr/local/bin:"* ]] && echo 'export PATH=$PATH:/usr/local/bin' >> /etc/profile
  cp mozartctl /usr/local/bin
  chmod 755 /usr/local/bin/mozartctl
elif [[ "$OSTYPE" == "darwin"* ]]; then
  cp mozartctl /usr/local/bin
  chmod 755 /usr/local/bin/mozartctl
else
  echo "Cannot detect OS."
fi
