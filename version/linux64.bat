set GO111MODULE=on
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-w -s" sshtunnel main.go

pause