# 3x-ui-amnezia Roadmap

Цель: добавить поддержку AmneziaWG / AmneziaWG 2.0 в панель 3x-ui, сохраняя существующую Xray-функциональность.

## Основные задачи

1. Анализ репозитория
   - Backend: контроллеры, сервисы, маршруты API
   - Frontend: страницы inbounds/xray, модель данных, статические файлы
   - Database: GORM, SQLite, AutoMigrate, текущие таблицы
   - Система установки: `install.sh`, `x-ui.service.*`
   - Конфигогенерация: `model.Inbound.GenXrayInboundConfig`, `XrayService.GetXrayConfig`

2. Добавить AmneziaWG архитектуру
   - Ввести новые модели данных для серверов, peers, трафика и настроек
   - Подключить их к авто-миграции
   - Обеспечить обратную совместимость со старой базой
   - Поддержку `migrate` и restore через существующие механизмы бэкапа

3. Создать backend API
   - `GET /panel/api/amnezia/servers`
   - `POST /panel/api/amnezia/servers`
   - `GET /panel/api/amnezia/servers/:id`
   - `PUT /panel/api/amnezia/servers/:id`
   - `DELETE /panel/api/amnezia/servers/:id`
   - `GET /panel/api/amnezia/servers/:id/peers`
   - `POST /panel/api/amnezia/servers/:id/peers`
   - `PUT /panel/api/amnezia/peers/:id`
   - `DELETE /panel/api/amnezia/peers/:id`
   - `POST /panel/api/amnezia/servers/:id/start`
   - `POST /panel/api/amnezia/servers/:id/stop`
   - `POST /panel/api/amnezia/servers/:id/restart`
   - `GET /panel/api/amnezia/peers/:id/config`
   - `GET /panel/api/amnezia/peers/:id/qrcode`
   - `GET /panel/api/amnezia/peers/:id/stats`

4. Service manager AmneziaWG
   - Установщик amneziawg-go / amneziawg-tools
   - Управление `amneziawg@<interface>.service`
   - Валидация конфигов и restart без перезапуска панели
   - Логи, статус и диагностика

5. UI
   - Добавить секцию `AmneziaWG` в меню
   - Реализовать таблицу серверов, клиентов, трафика, настроек, логов
   - Форма server/client с параметрами AmneziaWG 2.0
   - QR и скачивание config

6. Install script
   - Расширить `install.sh`: проверка root, OS, зависимости, backup, установка fork и AmneziaWG, systemd сервис, firewall
   - Включить backup в `/usr/local/x-ui/backup/`
   - Не перезаписывать существующую конфигурацию без backup

7. Тесты
   - Миграции базы данных
   - Создание серверов и peer
   - Генерация ключей и конфигов
   - QR-коды
   - Backup/restore
   - Установка на чистый Ubuntu/Debian

## Текущий фокус

- Добавить новые модели AmneziaWG и автo-миграцию
- Проверить, что `x-ui migrate` добавляет новые таблицы
- Обеспечить восстановление базы через существующий restore flow

## Минимальные критерии для текущего шага

- Файл модели `database/model/amnezia.go` создан
- `database/db.go` включает новые модели в `initModels()`
- `x-ui migrate` и импорт базы работают без ошибок
- Восстановление из резервной копии не ломает структуру базы
