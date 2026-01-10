# Letta Server

Простой HTTP сервер для определения публичного IP адреса клиента.

## Установка на VPS

```bash
# На вашем VPS сервере (45.130.214.133)
cd /opt
git clone <your-repo-url> letta-server
cd letta-server/server
go build -o letta-server ./cmd/letta-server

# Запуск (можно использовать systemd для автозапуска)
./letta-server
```

## Использование

Сервер запускается на порту 8080 и предоставляет endpoint:

```
GET http://45.130.214.133:8080/ip
```

Ответ:
```json
{
  "ip": "185.123.45.67"
}
```

## Автозапуск через systemd

Создайте файл `/etc/systemd/system/letta-server.service`:

```ini
[Unit]
Description=Letta IP Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/letta-server/server
ExecStart=/opt/letta-server/server/letta-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Затем:
```bash
sudo systemctl enable letta-server
sudo systemctl start letta-server
sudo systemctl status letta-server
```
