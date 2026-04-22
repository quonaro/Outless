## 🗺 Схема получения данных (Client Journey)

Эта схема описывает путь от ввода ссылки в v2rayN до момента, когда сотрудник начинает пользоваться интернетом через твой Хаб.

```mermaid
sequenceDiagram
    participant User as Сотрудник (v2rayN)
    participant API as Outless Subscription API
    participant DB as База Данных (PostgreSQL)
    participant Hub as Outless Hub (Xray + Go)
    participant Exit as Public/Corp Exit Node

    Note over User, API: 1. Получение списка серверов
    User->>API: GET /v1/sub/{token}
    API->>DB: Проверить токен и группу
    DB-->>API: Токен валиден (Group: DevOps)
    API->>DB: Выбрать TOP-50 живых нод (Latency < 500ms)
    DB-->>API: Список нод
    API-->>User: Base64 (VLESS конфиги с доменом hub.mirita.io)

    Note over User, Hub: 2. Установка соединения
    User->>Hub: TCP Handshake (hub.mirita.io:443)
    User->>Hub: TLS/Reality Handshake (SNI: google.com)
    Hub->>Hub: Проверка токена/UUID в памяти (Map)
    
    Note over Hub, Exit: 3. Ретрансляция
    Hub->>Exit: Proxying (VLESS UUID_Public)
    Exit-->>Hub: Response
    Hub-->>User: Data Flow
```

---

## 🏗 Структура ответа API (Что видит клиент)

Чтобы маскировка работала идеально, ИИ должен генерировать ссылки следующего вида:

| Поле | Значение (Пример) | Комментарий |
| :--- | :--- | :--- |
| **Address** | `hub.mirita.io` | Всегда твой сервер |
| **Port** | `443` | Стандартный HTTPS |
| **UUID** | `user-private-uuid` | Личный UUID сотрудника |
| **SNI** | `google.com` | Маскировка под Reality |
| **Path** | `/pl-1` или `user-id.pl` | Позволяет Хабу понять, куда слать трафик |
| **Remark** | `[PL] Outless-Public-1` | Красивое имя в клиенте |

---