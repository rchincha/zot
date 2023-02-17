#!/bin/sh

set -e
set -x

WORKDIR=$(mktemp -d)

REGISTRY_URL="http://localhost:8080"
REPO="dev.icr.io/oci-artifacts/test"

TAG="artifact"
JSON="$TAG.json"
echo '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"image/gif","digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size":2},"subject":{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:71dbae9d7e6445fb5e0b11328e941b8e8937fdd52465079f536ce44bb78796ed","size":406},"layers":[{"mediaType":"image/gif","digest":"sha256:725c49c527a83669901d00392768df9f653b1964a056c54232bc4c93003ddb48","size":3540101}]}' > "$WORKDIR/$JSON"
DIGEST=$(sha256sum "$WORKDIR/$JSON" | awk '{print "sha256:"$1}')
curl -i --data-binary @"$WORKDIR/$JSON" -H "Content-Type: application/vnd.oci.image.manifest.v1+json" -X PUT "$REGISTRY_URL/v2/$REPO/manifests/$TAG"


TAG="no-config"
JSON="$TAG.json"
echo '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","subject":{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:71dbae9d7e6445fb5e0b11328e941b8e8937fdd52465079f536ce44bb78796ed","size":406},"layers":[{"mediaType":"image/gif","digest":"sha256:725c49c527a83669901d00392768df9f653b1964a056c54232bc4c93003ddb48","size":3540101}]}' > "$WORKDIR/$JSON"
DIGEST=$(sha256sum "$WORKDIR/$JSON" | awk '{print "sha256:"$1}')
curl -i --data-binary @"$WORKDIR/$JSON" -H "Content-Type: application/vnd.oci.image.manifest.v1+json" -X PUT "$REGISTRY_URL/v2/$REPO/manifests/$TAG"

TAG="artifact-empty-layers"
JSON="$TAG.json"
echo '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"image/gif","digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size":2},"subject":{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:71dbae9d7e6445fb5e0b11328e941b8e8937fdd52465079f536ce44bb78796ed","size":406},"layers":[],"annotations":{"com.example.key1":"value1"}}' > "$WORKDIR/$JSON"
DIGEST=$(sha256sum "$WORKDIR/$JSON" | awk '{print "sha256:"$1}')
curl -i --data-binary @"$WORKDIR/$JSON" -H "Content-Type: application/vnd.oci.image.manifest.v1+json" -X PUT "$REGISTRY_URL/v2/$REPO/manifests/$TAG"

TAG="artifact-no-layers"
JSON="$TAG.json"
echo '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"image/gif","digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size":2},"subject":{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:71dbae9d7e6445fb5e0b11328e941b8e8937fdd52465079f536ce44bb78796ed","size":406},"annotations":{"com.example.key1":"value1"}}' > "$WORKDIR/$JSON"
DIGEST=$(sha256sum "$WORKDIR/$JSON" | awk '{print "sha256:"$1}')
curl -i --data-binary @"$WORKDIR/$JSON" -H "Content-Type: application/vnd.oci.image.manifest.v1+json" -X PUT "$REGISTRY_URL/v2/$REPO/manifests/$TAG"

TAG="artifact-no-subject"
JSON="$TAG.json"
echo '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"image/gif","digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size":2},"layers":[{"mediaType":"image/gif","digest":"sha256:725c49c527a83669901d00392768df9f653b1964a056c54232bc4c93003ddb48","size":3540101}]}' > "$WORKDIR/$JSON"
DIGEST=$(sha256sum "$WORKDIR/$JSON" | awk '{print "sha256:"$1}')
curl -i --data-binary @"$WORKDIR/$JSON" -H "Content-Type: application/vnd.oci.image.manifest.v1+json" -X PUT "$REGISTRY_URL/v2/$REPO/manifests/$TAG"

TAG="arifact-0-layer"
JSON="$TAG.json"
echo '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"image/gif","digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size":2},"subject":{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:71dbae9d7e6445fb5e0b11328e941b8e8937fdd52465079f536ce44bb78796ed","size":406},"layers":[{"mediaType":"application/octet-stream","digest":"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855","size":0}]}' > "$WORKDIR/$JSON"
DIGEST=$(sha256sum "$WORKDIR/$JSON" | awk '{print "sha256:"$1}')
curl -i --data-binary @"$WORKDIR/$JSON" -H "Content-Type: application/vnd.oci.image.manifest.v1+json" -X PUT "$REGISTRY_URL/v2/$REPO/manifests/$TAG"

TAG="artifact-scratch-layer"
JSON="$TAG.json"
echo '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"text/comment","digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size":2},"subject":{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:71dbae9d7e6445fb5e0b11328e941b8e8937fdd52465079f536ce44bb78796ed","size":406},"layers":[{"mediaType":"application/octet-stream","digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size":2}],"annotations":{"com.example.comment":"Hello world"}}' > "$WORKDIR/$JSON"
DIGEST=$(sha256sum "$WORKDIR/$JSON" | awk '{print "sha256:"$1}')
curl -i --data-binary @"$WORKDIR/$JSON" -H "Content-Type: application/vnd.oci.image.manifest.v1+json" -X PUT "$REGISTRY_URL/v2/$REPO/manifests/$TAG"
