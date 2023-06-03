#!/bin/bash

config_env=local
config_path=$(pwd)/config/migrations.yml

# Sample migrations file:
# local:
#   dialect: postgres
#   datasource: "host=localhost sslmode=disable dbname=oxygen user=oxygen password=oxygen"
#   dir: scripts/migrations
#   table: migrations

command=$1
shift
command_args=$@


if [[ $command == "new" ]]; then
  name=$1

  if [[ $name == "" ]]; then
    echo "Pass name as an argument like './migrate.sh new create_users_table'"
    exit 1
  fi
fi

# shellcheck disable=SC2086
sql-migrate $command -config="$config_path" -env="$config_env" $command_args