# Note: Intended to be run as "make cloud-cluster-tests"
#       Makefile target installs & checks all necessary tooling
#       Extra tools that are not covered in Makefile target needs to be added in verify_prerequisites()

load helpers_cloud
load helpers_wait

function setup_cloud_services() {
  awslocal s3 --region "us-east-2" mb s3://zot-storage-test
  awslocal dynamodb --region "us-east-2" create-table --table-name "BlobTable" --attribute-definitions AttributeName=Digest,AttributeType=S --key-schema AttributeName=Digest,KeyType=HASH --provisioned-throughput ReadCapacityUnits=10,WriteCapacityUnits=5
}

function launch_zot_server() {
  local zot_server_address=${1}
  local zot_server_port=${2}
  echo "Launching Zot server ${zot_server_address}:${zot_server_port}" >&3

  # Setup zot server
  local zot_root_dir=${BATS_FILE_TMPDIR}/zot
  local zot_config_file=${BATS_FILE_TMPDIR}/zot_config_${zot_server_address}_${zot_server_port}.json

  echo ${zot_root_dir} >&3

  mkdir -p ${zot_root_dir}

  cat > ${zot_config_file}<<EOF
{
    "distSpecVersion": "1.1.0",
    "storage": {
        "rootDirectory": "${zot_root_dir}",
        "dedupe": true,
        "remoteCache": true,
        "storageDriver": {
            "name": "s3",
            "rootdirectory": "/zot",
            "region": "us-east-2",
            "regionendpoint": "localhost:4566",
            "bucket": "zot-storage-test",
            "secure": false,
            "skipverify": false
        },
        "cacheDriver": {
            "name": "dynamodb",
            "endpoint": "http://localhost:4566",
            "region": "us-east-2",
            "cacheTablename": "BlobTable",
            "repoMetaTablename": "RepoMetadataTable",
            "imageMetaTablename": "ImageMetaTable",
            "repoBlobsInfoTablename": "RepoBlobsInfoTable",
            "userDataTablename": "UserDataTable",
            "apiKeyTablename":"ApiKeyTable",
            "versionTablename": "Version"
        }
    },
    "http": {
        "address": "${zot_server_address}",
        "port": "${zot_server_port}",
        "realm": "zot"
    },
    "cluster": {
      "members": [
        "127.0.0.1:9000",
        "127.0.0.1:9001"
      ],
      "hashKey": "loremipsumdolors"
    },
    "log": {
        "level": "debug",
        "output": "${BATS_FILE_TMPDIR}/zot-${zot_server_address}-${zot_server_port}.log"
    }
}
EOF
    zot_serve_strace ${zot_config_file}
    wait_zot_reachable ${zot_server_port}
}

# Setup function for single zot instance
function setup() {
    # Verify prerequisites are available
    if ! $(verify_prerequisites); then
        exit 1
    fi

    setup_cloud_services
    launch_zot_server 127.0.0.1 9000
    launch_zot_server 127.0.0.1 9001
}

function teardown() {
    local zot_root_dir=${BATS_FILE_TMPDIR}/zot
    zot_stop
    rm -rf ${zot_root_dir}
    awslocal s3 rb s3://"zot-storage-test" --force
    awslocal dynamodb --region "us-east-2" delete-table --table-name "BlobTable"
}

# TODO: figure out how to identify which node processed the query
@test "Check for successful response for image tags" {
    run skopeo --insecure-policy copy --dest-tls-verify=false \
        docker://debian:latest docker://localhost:9000/debian:latest
    [ "$status" -eq 0 ]
    curl -v http://localhost:9000/v2/debian/tags/list
    [ "$status" -eq 0 ]
}
