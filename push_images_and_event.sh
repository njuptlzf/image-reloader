#!/bin/bash
set -e

WEBHOOK_URL=${WEBHOOK_URL:-"http://localhost:8080/webhook"}
PUSH_CMD="$@"

if [ -z "$PUSH_CMD" ]; then
    echo "Error: Missing required environment variables."
    exit 1
fi

# 定义函数用于拆分镜像名称和标签
split_image_name_and_tag() {
  local input_string="$1"
  local imageName
  local imageTag
  
  # 检查是否包含":"
  if [[ "$input_string" == *":"* ]]; then
    # 从最后一个":"拆分
    imageName="${input_string%:*}"
    imageTag="${input_string##*:}"
  else
    # 从"@"拆分
    imageName="${input_string%@*}"
    imageTag="${input_string##*@}"
  fi

  echo "$imageName,$imageTag"
}

push_event() {
    IMAGE_NAME=$1
    NEW_TAG=$2
    # 构造CloudEvent数据
    EVENT_ID=$(date +%s)
    EVENT_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    PAYLOAD=$(cat <<EOF
{
    "resources": [
        {
            "digest": "sha256:dummy",
            "tag": "$NEW_TAG",
            "resource_url": "$IMAGE_NAME:$NEW_TAG"
        }
    ],
    "repository": {
        "name": "$IMAGE_NAME"
    }
}
EOF
    )
    echo "PAYLOAD=$PAYLOAD"

    # 发送CloudEvent到webhook
    echo "Sending CloudEvent to image-reloader webhook..."
    curl -X POST \
        -H 'Content-Type: application/json' \
        -d "$(jq -n --arg specversion "1.0" \
                        --arg id "$EVENT_ID" \
                        --arg source "https://${REGISTRY}/v2/${IMAGE_NAME}/tags/list" \
                        --arg type "io.github.container.image.pushed" \
                        --arg time "$EVENT_TIME" \
                        --arg datacontenttype "application/json" \
                        --argjson data "$PAYLOAD" \
                        '{specversion: $specversion, id: $id, source: $source, type: $type, time: $time, datacontenttype: $datacontenttype, data: $data}')" \
        "$WEBHOOK_URL"
    if [ $? -eq 0 ]; then
        echo "CloudEvent sent successfully."
    else
        echo "Failed to send CloudEvent."
    fi
}

echo ${PUSH_CMD}
# 推送镜像，这里以Docker为例，您可以根据需要替换为nerdctl push或其它命令
echo "Pushing image..."
eval "${PUSH_CMD}"

# 获取命令行参数并获取镜像字符串部分，假设该字符串的位置是参数的第三个位置
full_image=$(echo "$@" | awk '{print $3}')

result=$(split_image_name_and_tag "$full_image")
# 分割结果为 imageName 和 imageTag
imageName=$(echo "$result" | cut -d',' -f1)
imageTag=$(echo "$result" | cut -d',' -f2)
echo "ImageName: $imageName"
echo "ImageTag: $imageTag"
if [ -z "$imageName" ]; then
    echo "Error: Failed to extract image name."
    exit 1
fi
if [ -z "$imageTag" ]; then
    echo "Error: Failed to extract image tag."
    exit 1
fi

# 检查推送是否成功
if [ $? -eq 0 ]; then
    echo "push image cloudevnt to $WEBHOOK_URL"
    push_event $imageName $imageTag
else
    echo "Image push failed."
fi
