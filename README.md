# sshtunnel
go语言实现的一个SSH隧道端口转发程序

运行命令
```
sshtunnel.exe ./config.json
```

---
config.json
```json
[
	{
		"host": "10.0.0.123",
		"port": 22,
		"username": "sshuser",
		"password": "sshpass",
		"tunnels": [
			{
				"remote": {
					"host": "10.0.0.124",
					"port": 80
				},
				"local": {
					"host": "0.0.0.0",
					"port": 12480
				}
			},
			{
				"remote": {
					"host": "10.0.0.125",
					"port": 80
				},
				"local": {
					"host": "0.0.0.0",
					"port": 12580
				}
			}
		]
	}
]
```
