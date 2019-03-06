
# How to Build and Run a GOlang based OpenOLT Adapter 

Assuming the VOLTHA2.0 environment is made using the quickstart.md in voltha-go.

```
cd ~/source/voltha-openolt-adapter
```

Get the latest code changes
```
git pull
```
To build the docker image
```
make build
```
This will create the voltha-openolt-adapter-go docker image
```
$ docker images | grep openolt
voltha-openolt-adapter-go        latest              38688e697472        2 hours ago         37.3MB
```
In case the python voltha openolt adapter is started, stop the python voltha openolt docker container


To start the GOlang based OpenOLT adapter 

DOCKER_HOST_IP=<HOST-IP> docker-compose -f compose/adapters-openolt-go.yml up -d

The functionality of OLT activation can be verified through BBSIM
Follow the below steps to start BBSIM and provision it through VOLTHA-CLI
https://github.com/opencord/voltha-bbsim/blob/master/README.md
