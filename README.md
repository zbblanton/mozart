# Mozart

## Description
Container Orchestration Tool written in Golang. Similar to tools such as Kubernetes and Docker Swarm.

## Features
* Orchestrate containers across multiple nodes.
* Automatically move containers around to other workers if a worker dies.
* Automatically bring containers back up if they die.
* Easy one line agent setup to quickly deploy workers.
* Fully automated TLS communication between the master and the agents.
* Create services accounts that can used to access the API allowing for extensions/plugins.

## Getting Started
The fastest way to get started is to install all three of Mozart's components to the same host. To do this simply run the commands below:

NOTE: Make sure to put in your server host IP in INSERT_HOST_IP_HERE!

```
git clone https://github.com/zbblanton/mozart.git
cd mozart
chmod +x install_mozart.sh
sudo ./install_mozart.sh
sudo mozartctl cluster create --server INSERT_HOST_IP_HERE --name mozart
sudo cp /etc/mozart/mozart-config.json /etc/mozart/config.json
sudo systemctl start mozart-server
```
Next run
```
mozartctl cluster print
```
This will reprint out the docker run command you need to start the mozart-agent up. The command will look similar to this:
```
docker run --name mozart-agent -d --rm --privileged -v /var/run/docker.sock:/var/run/docker.sock -p 49433:49433 -e "MOZART_SERVER_IP=192.168.0.45" -e "MOZART_JOIN_KEY=pHrmesTNgAUrxiRru-S9MJkq4bWjTIGpz-LkkgsUIbuBygPvGVc76_F_EdIVvSjCvvKZqq3MZU7-C37st-B4A2pEN3l6D0Vimj0Qbj3jIkAcYBU3pP6qtODUvbuZizxqOdY2dL8sUuQUeFp2BVNC0tE2T12ONSXagMQlC0Iq6_A=" -e "MOZART_CA_HASH=NaBI2rUXXYG_b9c2AS3euxU_ZSygH990v2VpcfVi3Ac=" zbblanton/mozart-agent
```
NOTE: Make sure to change the INSERT_HOST_IP_HERE to your agent's IP address.

Below is an example config file to test out. 
```
{
    "Name": "test123",
    "Image": "nginx",
    "Env": [
        "TEST1=1234",
        "TEST2=abcd"
    ],
    "AutoRemove": true,
    "Privileged": false
}
```
Save this as something like config.json and then run:
```
sudo mozartctl run config.json
```

