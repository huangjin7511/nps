export GOPROXY=direct

sudo apt-get update
sudo apt-get install gcc-mingw-w64-i686 gcc-multilib
env GOOS=windows GOARCH=386 CGO_ENABLED=1 CC=i686-w64-mingw32-gcc go build -ldflags "-s -w -extldflags -static -extldflags -static" -buildmode=c-shared -o npc_sdk.dll cmd/npc/sdk.go
env GOOS=linux GOARCH=386 CGO_ENABLED=1 CC=gcc go build -ldflags "-s -w -extldflags -static -extldflags -static" -buildmode=c-shared -o npc_sdk.so cmd/npc/sdk.go
tar -czvf npc_sdk_old.tar.gz npc_sdk.dll npc_sdk.so npc_sdk.h

curl -k -L -o "/opt/st/alist-linux-musl-mipsle.tar.gz" "https://github.com/AlistGo/alist/releases/download/$tag/alist-linux-musl-mipsle.tar.gz" 
tar -xzvf /opt/st/alist-linux-musl-mipsle.tar.gz -C /opt/st
tar -xvf /opt/st/upx-3.95-amd64_linux.tar.xz -C /opt
chmod 777 /opt/upx-3.95-amd64_linux/upx
chmod 777 /opt/st/strip
          
CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
/opt/st/strip npc.exe
/opt/upx-3.95-amd64_linux/upx --lzma --best npc.exe
tar -czvf windows_386_client_old.tar.gz npc.exe conf/npc.conf conf/multi_account.conf


CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/npc/npc.go
/opt/st/strip npc.exe
/opt/upx-3.95-amd64_linux/upx --lzma --best npc.exe
tar -czvf windows_amd64_client_old.tar.gz npc.exe conf/npc.conf conf/multi_account.conf


CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
/opt/st/strip nps.exe
/opt/upx-3.95-amd64_linux/upx --lzma --best nps.exe
tar -czvf windows_amd64_server_old.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps.exe


CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-s -w -extldflags -static -extldflags -static" ./cmd/nps/nps.go
/opt/st/strip nps.exe
/opt/upx-3.95-amd64_linux/upx --lzma --best nps.exe
tar -czvf windows_386_server_old.tar.gz conf/nps.conf conf/tasks.json conf/clients.json conf/hosts.json conf/server.key  conf/server.pem web/views web/static nps.exe
