set GO111MODULE=on
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-w -s" -o sshtunnel.exe main.go

pause