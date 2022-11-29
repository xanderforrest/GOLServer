#!/bin/bash
yum update -y
yum -y install go
yum -y install SDL2-devel
echo 'export GOPATH="ec2-user/go"' >> ~/.bashrc
echo 'export PATH="$PATH:/usr/local/go/bin:$GOPATH/bin"' >> ~/.bashrc
source ~/.bashrc
git clone "https://github.com/xanderforrest/GOLServer" /home/ec2-user/GOLServer
cp /home/ec2-user/GOLServer/golengine.service /lib/systemd/system/golengine.service
chmod 644 /lib/systemd/system/golengine.service
systemctl daemon-reload
systemctl enable golengine.service
systemctl start golengine