# 🚀 Автоматический деплой

## Что происходит при `git push`

1. ✅ GitHub Actions подключается к серверу по SSH
2. ✅ Делает `git pull`
3. ✅ Запускает `docker compose up --build -d`

---

## Настройка GitHub Secrets

**GitHub → Settings → Secrets and variables → Actions** → добавь:

| Secret | Значение |
|--------|----------|
| `SSH_HOST` | IP сервера (например `1.2.3.4`) |
| `SSH_USER` | Имя пользователя (например `ubuntu`) |
| `SSH_PRIVATE_KEY` | Приватный SSH ключ (см. ниже) |
| `SSH_PORT` | Порт SSH (обычно `22`) |

### Генерация SSH ключа

```bash
# На локальной машине
ssh-keygen -t ed25519 -C "github-deploy" -f ~/.ssh/github-deploy

# Приватный ключ → GitHub Secret SSH_PRIVATE_KEY
cat ~/.ssh/github-deploy

# Публичный ключ → сервер
cat ~/.ssh/github-deploy.pub
```

```bash
# На сервере
mkdir -p ~/.ssh
echo "ВСТАВЬ_ПУБЛИЧНЫЙ_КЛЮЧ" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

---

## Первоначальная настройка сервера

```bash
# 1. Установи Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER
newgrp docker

# 2. Клонируй репозиторий
git clone https://github.com/YOUR_USERNAME/polymarket-bot.git
cd polymarket-bot

# 3. Настрой .env
cp .env.example .env
nano .env  # отредактируй токены и пароли

# 4. Запусти
docker compose up -d
```

---

## Команды на сервере

```bash
# Логи
docker compose logs -f bot

# Перезапуск
docker compose restart bot

# Остановка
docker compose down

# Полная пересборка
docker compose up --build -d
```

---

## Миграция БД (только если были данные ДО фикса)

⚠️ **Нужно ТОЛЬКО если в БД уже были записи со смешанным регистром адресов.**

Если БД пустая или пересоздаётся — миграция **НЕ нужна**.

```bash
# Войди в контейнер PostgreSQL
docker compose exec postgres psql -U whale_user -d polymarket

# Нормализуй адреса к lowercase
UPDATE watchlist SET wallet = LOWER(wallet);
\q
```

---

## Troubleshooting

**Бот не запускается:**
```bash
docker compose logs bot
```

**PostgreSQL проблемы:**
```bash
docker compose logs postgres
docker compose exec postgres psql -U whale_user -d polymarket
```

**Redis проблемы:**
```bash
docker compose logs redis
docker compose exec redis redis-cli ping
```

**Пересоздать всё с нуля:**
```bash
docker compose down -v  # удалит volumes!
docker compose up -d
```
