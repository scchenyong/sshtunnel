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
		"addr": "10.0.0.123:22",
		"user": "sshuser",
		"pass": "sshpass",
		"tunnels": [
			{
				"isInput": true,
				"remote": "10.0.0.111:58000",
				"local": "127.0.0.1:8000"
			},
			{
				"remote": "127.0.0.1:8000",
				"local": "0.0.0.0:58000"
			}
		]
	}
]
```

## 配置说明
配置文件采用JSON文件格式，支持多主机和多转发
- addr: 需要开启SSH隧道的主机地址，格式为【IP地址:端口】
- user: 主机访问用户名
- pass: 主机访问密码，可选配置（未配置时程序将通过控制台输入）
- tunnels: 包含的隧道转发
  - isInput: 开启远端输入，需开启配置/etc/ssh/ssh_config：AllowTcpForwarding=yes,GatewayPorts=yes
  - remote: 开启隧道的远程主机配置，格式为【IP地址:端口】
  - local: 开启隧道映射到本地的配置，格式为【IP地址:端口】
