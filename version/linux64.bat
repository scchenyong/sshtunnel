set GO111MODULE=on
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-w -s" -o sshtunnel_linux_amd64 main.go

pause