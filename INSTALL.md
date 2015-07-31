# 运维
## 1. zeromq的安装:
注意一下两个关键目录:
* /usr/local/lib/
* /usr/lib64/

### 安装步骤:
* 在Ubuntu上

```bash
# 下载: libsodium 和 zeromq
tar -zxvf libsodium_1.0.3.orig.tar.gz
cd libsodium-1.0.3/
./configure --libdir=/usr/local/lib && make check
make && make install
cd ..

tar -zxvf zeromq-4.1.2.tar.gz && cd zeromq-4.1.2
./configure --libdir=/usr/local/lib
make && make install

```
* 在CentOs6.5上

```bash
yum install libsodium.x86_64 libsodium-devel.x86_64 -y

tar -zxvf zeromq-4.1.2.tar.gz && cd zeromq-4.1.2
./configure --libdir=/usr/lib64/
make && make install
```


## 2. Go的安装（可选)
参考： https://golang.org/doc/install
安装go

```bash
 # 下载: go1.4.2.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.4.2.linux-amd64.tar.gz
vim /etc/profile
# 添加
export PATH=$PATH:/usr/local/go/bin

# 添加一个独立的用户或利用已有的用户
adduser rpc
su - rpc

# 设置GOPATH
vim .bash_profile
export GOPATH=/home/rpc/workspace

# 下载代码&下载依赖
cd /home/rpc/workspace
go get -u github.com/tools/godep
go install github.com/tools/godep
go get -u github.com/wfxiang08/rpc_proxy

# 下载依赖
cd /home/rpc/workspace/src/github.com/wfxiang08/rpc_proxy
/home/rpc/workspace/bin/godep restore
```