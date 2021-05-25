#! /bin/bash
set -e

pg_version=12.5
build_id=$(date +%Y%m%d)
version="$pg_version-$build_id"

tags=("flyio/postgres-ha:$version" "flyio/postgres-ha:$pg_version")

echo ${tags[@]}

tags_args=""

for tag in ${tags[@]};
do
  tags_args="$tags_args --tag $tag"
done

# exit 0

iamge_id=$(docker build . --build-arg VERSION=$version --build-arg PG_VERSION=$pg_version $tags_args)

for tag in ${tags[@]};
do
  echo "pushing $tag"
  docker push $tag
done