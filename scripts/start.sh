#! /bin/bash
set -e

ip=`getent hosts fly-local-6pn | cut -d ' ' -f1`
if [[ "$ip" == "" ]]; then
  ip="127.0.0.1"
fi

if [ -z ${FLY_APP_NAME} ]; then
    FLY_APP_NAME="local"
fi

if [[ "$ip" != "127.0.0.1" ]]; then
    node_id=$(echo $ip | sed "s/^fdaa:0:1:a7b://" | sed "s/://g" )
fi

if [ -z ${node_id} ]; then
    node_id="local"
fi

consul_node="$(echo $FLY_CONSUL_URL | sed -r 's/https:\/\/[^\/]+\///')$node_id"

mkdir -p /data

# because stolon needs entirely different sets of env vars :/
cat <<EOF > /data/.env
STOLONCTL_CLUSTER_NAME=$FLY_APP_NAME
STOLONCTL_STORE_BACKEND=consul
STOLONCTL_STORE_URL=$FLY_CONSUL_URL
STOLONCTL_STORE_NODE=$consul_node

EOF

sed 's/STOLONCTL_/STKEEPER_/' /data/.env >> /data/.env
sed 's/STOLONCTL_/STSENTINEL_/' /data/.env >> /data/.env
sed 's/STOLONCTL_/STPROXY_/' /data/.env >> /data/.env

# write stolon initial cluster spec
cat <<EOF > /fly/initial-cluster-spec.json
{"initMode":"new"}
EOF

su_password="${SU_PASSWORD:-supassword}"
repl_password="${REPL_PASSWORD:-replpassword}"
primary_region="${PRIMARY_REGION}"

pg_proxy_port="${PG_PROXY_PORT:-5432}"
pg_port="${PG_PORT:-5433}"

keeper_options="--uid $node_id --data-dir /data/ --pg-su-username=flypgadmin --pg-repl-username=repluser --pg-listen-address=$ip --pg-port $pg_port --log-level warn"

if [ "$primary_region" != "" ]; then
    if [ "$primary_region" != "$FLY_REGION" ]; then
        keeper_options="$keeper_options --can-be-master=false --can-be-synchronous-replica=false"
    fi
fi


export STKEEPER_PG_SU_PASSWORD=$su_password
export STKEEPER_PG_REPL_PASSWORD=$repl_password

# write procfile for hivemind
cat << EOF > /fly/Procfile
keeper: stolon-keeper $keeper_options
sentinel: stolon-sentinel --initial-cluster-spec /fly/initial-cluster-spec.json
proxy: stolon-proxy --listen-address=$ip --port=$pg_proxy_port --log-level=warn
postgres_exporter: DATA_SOURCE_URI=[$ip]:$pg_port/postgres?sslmode=disable DATA_SOURCE_PASS=$SU_PASSWORD DATA_SOURCE_USER=flypgadmin PG_EXPORTER_EXCLUDE_DATABASE=template0,template1 PG_EXPORTER_DISABLE_SETTINGS_METRICS=true PG_EXPORTER_AUTO_DISCOVER_DATABASES=true PG_EXPORTER_EXTEND_QUERY_PATH=/fly/queries.yaml postgres_exporter
EOF

chown -R stolon:stolon /data/
cd /data/
rm -f .overmind.sock

export OVERMIND_NO_PORT=1
export OVERMIND_AUTO_RESTART=sentinel,proxy
export OVERMIND_STOP_SIGNALS="keeper=TERM"
export OVERMIND_TIMEOUT=300
exec gosu stolon overmind start -f /fly/Procfile