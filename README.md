# sshtunnel
go语言实现的一个SSH隧道端口转发程序

## 运行命令
```
sshtunnel.exe ./config.json
```

## 配置示例
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

## 配置说明
配置文件采用JSON文件格式，支持多主机和多转发
- host: 需要开启隧道的主机地址
- port: 对应主机的SSH协议端口
- username: 主机访问用户名
- password: 主机访问密码
- tunnels: 包含的隧道转发
	- remote: 开启隧道的远程主机配置
	- local: 开启隧道映射到本地的配置
