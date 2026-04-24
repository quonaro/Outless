# Outless Configuration

## Обзор

Файл `outless.yaml` конфигурирует все компоненты embedded backend: базу данных, HTTP API, monitor, router и Xray runtime.

## Текущая структура

```yaml
# База данных
database:
  url: "postgres://outless:outless@postgres:5432/outless?sslmode=disable"

# JWT конфигурация
jwt:
  secret: "..."                  # Секрет для токенов
  expiry: "24h"                  # Срок действия

# Admin конфигурация
admin:
  login: ""                      # Логин первого админа (env bootstrap)
  password: ""                   # Пароль первого админа (env bootstrap)

# HTTP API сервер
api:
  shutdown: "10s"               # Время для корректного завершения

# Monitor - сервис мониторинга доступности узлов
monitor:
  workers: 16                    # Количество горутин для проверки
  refresh_interval: "10m"       # Интервал обновления public sources
  poll_interval: "5s"            # Интервал опроса заданий проверки
  check_interval: "10m"          # Интервал полной проверки всех узлов
  geoip:
    db_path: "/app/tmp/GeoLite2-Country.mmdb"
    db_url: "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb"
    auto: true                    # Автообновление
    ttl: "24h"                   # Интервал обновления
  agents:
    workers: 2                   # Количество агентов
    url: "https://www.google.com/generate_204"  # URL для проверки

# Router - Xray edge (router-facing)
router:
  port: 443                      # Порт для Reality inbound
  sni: "www.google.com"           # SNI для Reality
  public_key: "..."              # Публичный ключ Reality
  private_key: "..."             # Приватный ключ Reality
  short_id: "..."                # Short ID для Reality
  fingerprint: "chrome"          # TLS fingerprint
  address: ":443"                # Адрес для прослушивания
  sync_interval: "5s"           # Интервал синхронизации конфига с БД
```

## Ответственность секций

### `database`
- **Обязательна:** Да
- **Описание:** Подключение к PostgreSQL
- **Используется:** Всеми сервисами (API, Router, Monitor)
- **Примечание:** Только PostgreSQL поддерживается

### `jwt`
- **Обязательна:** Да
- **Описание:** Конфигурация JWT токенов
- **Используется:** API сервисом
- **Ключевые поля:**
  - `secret` - для подписи токенов
  - `expiry` - срок действия

### `admin`
- **Обязательна:** Да
- **Описание:** Конфигурация первого админа
- **Используется:** API сервисом при bootstrap
- **Ключевые поля:**
  - `login/password` - учётные данные первого админа

### `api`
- **Обязательна:** Да
- **Описание:** Конфигурация HTTP API сервера
- **Используется:** HTTP API сервисом
- **Ключевые поля:**
  - `shutdown` - время для корректного завершения работы

### `monitor`
- **Обязательна:** Да
- **Описание:** Конфигурация сервиса мониторинга доступности узлов
- **Используется:** Monitor сервисом
- **Ключевые поля:**
  - `workers` - параллелизм проверок
  - `poll_interval` - как часто проверять очередь заданий
  - `check_interval` - как часто проверять все узлы
  - `geoip.*` - настройки GeoIP для определения страны
  - `agents.workers` - количество агентов
  - `agents.url` - URL для проверки доступности

### `router`
- **Обязательна:** Да
- **Описание:** Конфигурация Xray edge instance (router-facing)
- **Используется:** Router менеджером
- **Ключевые поля:**
  - `port`, `sni`, `public_key`, `private_key` - параметры Reality
  - `sync_interval` - как часто синхронизировать конфиг с БД

