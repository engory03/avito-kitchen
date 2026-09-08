# Авито.Кухня

MVP API-площадки доставки еды: каталог, корзина с бронью порции, заказ и закрытый API для заведений. Клиента нет. Пользователя отличаем по заголовку `X-User-Id` (строка, не сессия).

Человек выбирает заведение, кладёт блюдо в корзину (позиция **бронируется на 10 минут**), проверяет корзину и оформляет заказ. Заведение подтверждает или отклоняет. Если молчит дольше **15 минут** — заказ сам становится `cancelled`.

Документация — в `[docs/](docs/README.md)`.

## Документация


|                  |                                                                                                      |
| ---------------- | ---------------------------------------------------------------------------------------------------- |
| CJM пользователя | [docs/cjm/user-cjm.puml](docs/cjm/user-cjm.puml)                                                     |
| CJM заведения    | [docs/cjm/restaurant-cjm.puml](docs/cjm/restaurant-cjm.puml)                                         |
| C4 контейнеры    | [docs/architecture/c4-containers.puml](docs/architecture/c4-containers.puml)                         |
| C4 компоненты    | [docs/architecture/c4-components.puml](docs/architecture/c4-components.puml)                         |
| Таблицы          | [docs/architecture/db.md](docs/architecture/db.md)                                                   |
| ER               | [docs/architecture/er.puml](docs/architecture/er.puml)                                               |
| Миграции         | [migrations/000001_init.up.sql](migrations/000001_init.up.sql)                                       |
| OpenAPI          | [api/openapi.yaml](api/openapi.yaml)                                                                 |
| Решения          | [docs/design-decisions.md](docs/design-decisions.md)                                                 |
| API              | [docs/api/endpoints.md](docs/api/endpoints.md)                                                       |
| Стыки фич        | [docs/api/edge-cases.md](docs/api/edge-cases.md)                                                     |
| Бронь            | [docs/architecture/sequence-add-to-cart.puml](docs/architecture/sequence-add-to-cart.puml)           |
| Заказ            | [docs/architecture/sequence-validate-confirm.puml](docs/architecture/sequence-validate-confirm.puml) |
| Кэш каталога     | [docs/architecture/sequence-catalog-cache.puml](docs/architecture/sequence-catalog-cache.puml)       |
| Swagger UI       | [docs/swagger/](docs/swagger/README.md)                                                              |
| Промпт           | [PROMPT.md](PROMPT.md)                                                                               |


`.puml` можно открыть на [plantuml.com](https://www.plantuml.com/plantuml/uml/) или плагином в IDE.

## Что внутри

Два процесса в [docker-compose.yml](docker-compose.yml). Остатки и заказы живут в Postgres. Redis — кэш витрины; если он недоступен, каталог читается из БД.


| Сервис         | Порт | Зачем                                                             |
| -------------- | ---- | ----------------------------------------------------------------- |
| `kitchen-api`  | 8080 | Площадка: каталог, корзина, заказы, partner API                   |
| `partner-demo` | 8081 | Одно заведение: меню и сток **в памяти**, notify → confirm/reject |
| PostgreSQL     | 5432 | Меню, брони, корзины, заказы                                      |
| Redis          | 6379 | Кэш витрины (не брони и не заказы)                                |


При старте `partner-demo` сам делает `PUT /api/v1/partner/menu`. В каталоге появляется «Пельменная у Авито».

## Как запустить

Нужен **Docker Desktop**. Из корня репозитория.

### Поднять стек

```bash
docker compose up -d --build
```

То же через обёртки (одинаковые цели в [Makefile](Makefile) и [tasks.ps1](tasks.ps1)):

```bash
make up
```

```powershell
.\tasks.ps1 up
```

`.\tasks.ps1 help` / `make help` — полный список команд.

Первый запуск собирает образы Go (~минута). Миграции применяет `kitchen-api` при старте.

### Проверить, что живы

```bash
curl.exe http://localhost:8080/health
curl.exe http://localhost:8081/health
curl.exe http://localhost:8080/api/v1/restaurants
```

В PowerShell `curl` — это `Invoke-WebRequest`. Для сырого HTTP удобнее `curl.exe` или `Invoke-RestMethod`.

Ожидается: оба health = `ok`, в списке заведений — пельменная. Если список пустой — подождите пару секунд и повторите или `.\tasks.ps1 seed`.

### Swagger UI

После старта `kitchen-api`: [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html).

1. **Authorize** (замок сверху):
  - `UserID` → `X-User-Id`, любая строка, например `anna`
  - `ApiKey` → `X-Api-Key`, для демо `demo-partner-key`
2. Витрина (`GET /restaurants`, меню) открыта без заголовков.
3. Корзина, заказы и partner без заголовка отвечают **401**.

Спека: `[api/openapi.yaml](api/openapi.yaml)`. UI — из комментариев в коде, папка `[docs/swagger](docs/swagger/README.md)`. После правок хендлеров:

```bash
make swag
```

```powershell
.\tasks.ps1 swag
```

Потом пересоберите `kitchen-api` (`make up` / `.\tasks.ps1 up`), UI берёт схему из бинарника.

### Остановить / заново залить меню

```bash
make down          # остановить контейнеры
make seed          # restart partner-demo → снова PUT меню
```

```powershell
.\tasks.ps1 down
.\tasks.ps1 seed
```

`docker compose down` не удаляет volume Postgres. Полностью сбросить БД: `docker compose down -v`.

### Локально без Compose

Нужны уже запущенные Postgres и Redis, затем `[.env.example](.env.example)` → `.env`:

```bash
cp .env.example .env
go run ./cmd/kitchen-api
go run ./cmd/partner-demo
```

```powershell
Copy-Item .env.example .env
go run ./cmd/kitchen-api
go run ./cmd/partner-demo
```

В `.env` для локального запуска: `KITCHEN_BASE_URL=http://localhost:8080`, `PARTNER_BASE_URL` / `PARTNER_PUBLIC_URL=http://localhost:8081`.

Собрать бинарники в `bin/`: `make build` / `.\tasks.ps1 build`.

## Команды


| Действие                | Linux / macOS           | Windows (PowerShell)           |
| ----------------------- | ----------------------- | ------------------------------ |
| Поднять Compose         | `make up`               | `.\tasks.ps1 up`               |
| Остановить              | `make down`             | `.\tasks.ps1 down`             |
| Заново залить демо-меню | `make seed`             | `.\tasks.ps1 seed`             |
| Генерация Swagger       | `make swag`             | `.\tasks.ps1 swag`             |
| Юнит + покрытие         | `make test`             | `.\tasks.ps1 test`             |
| Интеграция (Docker)     | `make test-integration` | `.\tasks.ps1 test-integration` |
| E2E (Docker)            | `make test-e2e`         | `.\tasks.ps1 test-e2e`         |
| Нагрузка каталога (k6)  | `make loadtest-smoke`   | `.\tasks.ps1 loadtest-smoke`   |
| Линтер                  | `make lint`             | `.\tasks.ps1 lint`             |
| kitchen-api локально    | `make dev`              | `.\tasks.ps1 dev`              |


Раннер тестов — `go tool gotestsum` из `go.mod`, отдельно ставить не нужно.

## Архитектура

Два контейнера приложения: площадка и демо-заведение. Клиента нет. Заведение ходит с `X-Api-Key`; пользователь — с `X-User-Id`.

- Картинка сервисов: [C4 Level 2](docs/architecture/c4-containers.puml)
- Внутри `kitchen-api`: [C4 Level 3](docs/architecture/c4-components.puml)
- Почему Redis не заменяет бронь: [design-decisions.md](docs/design-decisions.md)

Четыре разных «часа», не один TTL: бронь корзины 10 мин (Postgres), ожидание заведения 15 мин (`confirm_until`), HTTP notify 2 сек, кэш витрины минуты (Redis).

## База данных

Остатки и брони хранит **площадка**. У `partner-demo` своей БД нет — сток в памяти процесса.

Главные правила в полях:

- свободный остаток = `dishes.stock_qty` минус живые брони (`dish_reservations`, `active` и `expires_at > now()`)
- одна корзина на `user_id` и сразу одно заведение
- заказ ждёт заведение до `orders.confirm_until`
- повторный `POST /orders` с тем же `Idempotency-Key` не создаёт второй заказ
- имя и цена в заказе скопированы (`order_items`), смена статуса пишется в `order_status_history`

Таблицы: [docs/architecture/db.md](docs/architecture/db.md).  
Картинка: [docs/architecture/er.puml](docs/architecture/er.puml).  
SQL: [migrations/000001_init.up.sql](migrations/000001_init.up.sql).

## Сценарии API

Клиент: заголовок `X-User-Id`. Заведение: `X-Api-Key`. Базовый путь `/api/v1`.

1. `GET /restaurants` и `.../menu` — витрина; `available_qty` всегда из Postgres
2. `POST /cart/items` — бронь 10 минут; чужое заведение → сначала `DELETE /cart`
3. `POST /cart/validate` — `ok` / `partial` / `invalid`, заказ ещё не создаётся
4. `POST /orders` + `Idempotency-Key` — проверка заново, списание, `confirm_until` +15 мин, notify ≤ 2 сек
5. `GET /orders/{id}` — статус и `status_history`; если дедлайн вышел — уже `cancelled`
6. Отмена пользователем — только из `created` / `pending_confirmation`

Полный список: [docs/api/endpoints.md](docs/api/endpoints.md).  
Стыки (два человека на последнюю порцию, Redis упал, заведение молчит): [docs/api/edge-cases.md](docs/api/edge-cases.md).

## Тесты

Из корня. Юнит — без Docker. Интеграция и e2e поднимают **Postgres через testcontainers** (нужен Docker Desktop). На Windows testcontainers иногда пропускает тесты (`rootless Docker is not supported`) — это среда, не сломанный код.

```bash
make test                 # юнит, без ./e2e; coverage.out
make test-integration     # -tags=integration; coverage-integration.out
make test-e2e             # HTTP: меню → корзина → заказ → replay ключа
```

```powershell
.\tasks.ps1 test
.\tasks.ps1 test-integration
.\tasks.ps1 test-e2e
```

Без обёрток:

```bash
go test ./internal/domain ./internal/cart ./internal/httpserver ./internal/partnerdemo
go test -tags=integration -timeout 8m ./internal/cart ./internal/order
go test -timeout 8m ./e2e/
```

`go test ./...` без тега `integration` затронет и `./e2e` (у e2e нет build-tag). Юнит без e2e — команды выше, не `./...`.

E2E против уже поднятого стека: `E2E_BASE_URL=http://localhost:8080` и `.\tasks.ps1 test-e2e`.

Нагрузка витрины (стек должен быть `up`, нужен [k6](https://grafana.com/docs/k6/latest/set-up/install-k6/) или образ):

```bash
make loadtest-smoke
make loadtest              # BASE_URL, LOADTEST_RATE, LOADTEST_DURATION
make loadtest-docker       # grafana/k6, без локального k6
```

Скрипт: [loadtest/k6/catalog.js](loadtest/k6/catalog.js).

## Качество кода

Линтер: [golangci-lint](https://golangci-lint.run/), конфиг в репозитории — `[.golangci.yml](.golangci.yml)`.

```bash
make lint
make lint-fix
```

```powershell
.\tasks.ps1 lint
.\tasks.ps1 lint-fix
```

Стек: Go, PostgreSQL, Redis, Docker Compose, chi, pgx, swaggo.

## Упрощения

- Нет логина пользователя, только `X-User-Id`
- `delivery_address` — строка для показа, не курьер
- Просроченные брони и `confirm_until` проверяем в момент запроса, без отдельного воркера
- В демо одно заведение; меню и сток у него в памяти, без своей БД
- Очереди уведомлений ресторану нет: HTTP ≤ 2 сек, иначе `pending_confirmation` и pull `GET /partner/orders`

Почему так: [docs/design-decisions.md](docs/design-decisions.md).