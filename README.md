
# How to Build and Run a GOlang based OpenOLT Adapter 
Assuming the VOLTHA2.0 environment is made using the quickstart.md in voltha-go.

Dependencies are committed to the repos as per current standard practice.
If you need to update them you can do so with dep. This includes the voltha-protos and voltha-go dependencies.

Ensure your GOPATH variable is set according to the quickstart
Create a symbolic link in the $GOPATH/src tree to the voltha-openolt-adapter repository:
```sh
mkdir -p $GOPATH/src/github.com/opencord
ln -s ~/repos/voltha-openolt-adapter $GOPATH/src/github.com/opencord/voltha-openolt-adapter
```

Install dep for fetching dependencies
```sh
mkdir -p $GOPATH/bin
curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

Pull and build dependencies for the project.  This may take a while.  This may likely update versions of 3rd party packages in the vendor folder.   This may introduce unexpected changes.   If everything still works feel free to check in the vendor updates as needed.

cd $GOPATH/src/github.com/opencord/voltha-go/
dep ensure

If you are using a custom local copy of protos or voltha-go

Just export LOCAL_PROTOS=true or LOCAL_VOLTHAGO=true to use that instead, assuming you have set them up on GOPATH. See the quickstart.

mkdir -p ~/source
cd ~/source
git clone https://github.com/opencord/voltha-openolt-adapter.git
cd voltha-openolt-adapter
make build
```
This will create the voltha-openolt-adapter-go docker image
```

$ docker images | grep openolt
voltha-openolt-adapter          latest               a0f2ba883655        5 seconds ago        842MB
voltha-openolt-adapter          latest-py            a0f2ba883655        5 seconds ago        842MB
voltha-openolt-adapter          latest-go            801070129648        About a minute ago   34.5MB

```
In case the python voltha openolt adapter is started, stop the python voltha openolt docker container


To start the GOlang based OpenOLT adapter 
```
cd ~/source/voltha-go
docker-compose -f compose/system-test-openolt-go.yml up -d
```
To start the Python based adapter based OpenOLT adapter 
```
cd ~/source/voltha-go
docker-compose -f compose/system-test.yml up -d
```
The functionality of OLT activation can be verified through BBSIM
Follow the below steps to start BBSIM and provision it through VOLTHA-CLI
https://github.com/opencord/voltha-bbsim/blob/master/README.md

