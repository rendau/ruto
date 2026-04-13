# ruto

### DB dump:

```
pg_dump --no-owner -Fc -U postgres ruto -f ./ruto.custom
```

### DB restore:

```
dropdb -U postgres ruto
createdb -U postgres ruto
pg_restore --no-owner -d ruto -U postgres ./ruto.custom
```

### Install `migrate` command-tool:

https://github.com/golang-migrate/migrate/tree/master/cmd/migrate

### Create new migration:

```
migrate create -ext sql -dir migrations mg_name
```

### Apply migration:

```
migrate -path migrations -database "postgres://localhost:5432/db_name?sslmode=disable" up
```
