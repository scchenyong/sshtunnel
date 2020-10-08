set GO111MODULE=on
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-w -s" -o sshtunnel_windows_amd64.exe main.go

pause