# Swagger UI

Сгенерировано из комментариев на хендлерах (`docs.go`, `swagger.json`, `swagger.yaml`). После правок аннотаций:

```bash
make swag
```

```powershell
.\tasks.ps1 swag
```

UI: http://localhost:8080/swagger/index.html (стек должен быть запущен).

В Authorize:

| | Заголовок | Пример |
|---|-----------|--------|
| пользователь | `X-User-Id` | `anna` |
| заведение | `X-Api-Key` | `demo-partner-key` |

Без заголовка корзина, заказы и partner отвечают 401. Каталог открыт.

Отдельная OpenAPI-спека: [`api/openapi.yaml`](../../api/openapi.yaml).
