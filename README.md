# Mozart

## Description
Container Orchestration Tool

## Getting Started
The fastest way to get started is to install all three of Mozart's components to the same host. To do this simply run the commands below:

NOTE: Make sure to put in your host IP in INSERT_HOST_IP_HERE!

```
git clone https://github.com/zbblanton/mozart_alpha.git
cd mozart_alpha
chmod +x install_mozart.sh
sudo ./install_mozart.sh
sudo mozartctl cluster create --server INSERT_HOST_IP_HERE --name mozart
sudo cp /etc/mozart/mozart-config.json /etc/mozart/config.json
sudo systemctl start mozart-server
```
Next run
```
mozartctl cluster install
```
This will print out the docker run command you need to start the mozart-agent up.
