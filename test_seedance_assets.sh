#!/bin/bash
# https://ark-doc.tos-ap-southeast-1.bytepluses.com/doc_image/r2v_tea_pic2.jpg
# 配置参数
BASE_URL="http://localhost:3002"
# API_KEY="${API_KEY:-sk-xxxx}" # 替换为你的 API Key，或者通过环境变量传入
# API_KEY="${API_KEY:-sk-WRvuLUp78vrU8L8kkEt4cj1xkAZ7yISRS6OJT7byPlrlVmnh}" # 替换为你的 API Key，或者通过环境变量传入
API_KEY="${API_KEY:-sk-WRvuLUp78vrU8L8kkEt4cj1xkAZ7yISRS6OJT7byPlrlVmnh}" # 替换为你的 API Key，或者通过环境变量传入

echo "=========================================="
echo "开始测试 Seedance 素材代理接口"
echo "后端地址: $BASE_URL"
echo "API Key: $API_KEY"
echo "=========================================="


# # 0. 测试渠道连通性
# echo -e "\n[0/4] 测试: 渠道连通性 (GET /api/channel/test/5)"
# CHANNEL_TEST_RES=$(curl -s -X GET "$BASE_URL/api/channel/test/5?model=dreamina-seedance-2-0-fast-260128" \
#   -H "Authorization: Bearer $API_KEY")
# echo "返回结果: $CHANNEL_TEST_RES"

# exit 0

# # 1. 测试创建分组接口
# echo -e "\n[1/4] 测试: 创建素材分组 (POST /v1/assets/groups/create)"
# GROUP_RES=$(curl -s -X POST "$BASE_URL/v1/assets/groups/create" \
#   -H "Authorization: Bearer $API_KEY" \
#   -H "Content-Type: application/json" \
#   -d '{
#     "name": "test-group-0011", 
#     "description": "Test group from script",
#   }')

# echo "返回结果: $GROUP_RES"

# 提取 Group ID
# GROUP_ID=$(echo "$GROUP_RES" | grep -o '"id":"[^"]*' | grep -o '[^"]*$')
# GROUP_ID="group-20260408173943-m2bxd"

# if [ -z "$GROUP_ID" ]; then
#     echo "❌ 提取 Group ID 失败，可能是接口请求出错，停止后续测试。"
#     exit 1
# fi
# echo "✅ 成功获取 Group ID: $GROUP_ID"

# # 2. 测试通过 URL 上传素材
# echo -e "\n[2/4] 测试: 通过 URL 上传素材 (POST /v1/assets/create)"
# ASSET_URL_RES=$(curl -s -X POST "$BASE_URL/v1/assets/create" \
#   -H "Authorization: Bearer $API_KEY" \
#   -H "Content-Type: application/json" \
#   -d "{
#     \"groupId\": \"$GROUP_ID\",
#     \"url\": \"https://images.unsplash.com/photo-1506744626753-1fa44df14c28?w=800&q=80\",
#     \"assetType\": \"Image\",
#     \"name\": \"test-url-image\",
#     \"model\": \"dreamina-seedance-2-0-260128\"
#   }")

# echo "返回结果: $ASSET_URL_RES"

# # 提取 Asset ID
# ASSET_ID=$(echo "$ASSET_URL_RES" | grep -o '"id":"[^"]*' | grep -o '[^"]*$')
# if [ -n "$ASSET_ID" ]; then
#     echo "✅ 成功获取 Asset ID: $ASSET_ID"
# else
#     echo "⚠️ 提取 Asset ID 失败"
# fi

ASSET_ID="asset-20260408174237-k2729"

# 3. 测试查询素材状态
if [ -n "$ASSET_ID" ]; then
    echo -e "\n[3/4] 测试: 查询素材状态 (POST /v1/assets/get)"
    ASSET_STATUS_RES=$(curl -s -X POST "$BASE_URL/v1/assets/get" \
      -H "Authorization: Bearer $API_KEY" \
      -H "Content-Type: application/json" \
      -d "{
        \"id\": \"$ASSET_ID\",
        \"model\": \"dreamina-seedance-2-0-260128\"
      }")
    echo "返回结果: $ASSET_STATUS_RES"
else
    echo -e "\n[3/4] 测试: 查询素材状态 (跳过，未获取到 Asset ID)"
fi

# 4. 测试本地文件上传素材
echo -e "\n[4/4] 测试: 本地文件上传素材 (POST /v1/assets/upload)"

# 创建一个测试用的图片文件
TEST_IMG_PATH="/tmp/test-seedance-upload.jpg"
echo "/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////wgALCAABAAEBAREA/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPxA=" | base64 -D > "$TEST_IMG_PATH" 2>/dev/null || echo "/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////wgALCAABAAEBAREA/8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPxA=" | base64 -d > "$TEST_IMG_PATH"

ASSET_FILE_RES=$(curl -s -X POST "$BASE_URL/v1/assets/upload" \
  -H "Authorization: Bearer $API_KEY" \
  -F "file=@$TEST_IMG_PATH" \
  -F "groupId=$GROUP_ID" \
  -F "assetType=Image" \
  -F "name=test-file-image" \
  -F "model=dreamina-seedance-2-0-260128")

echo "返回结果: $ASSET_FILE_RES"

echo -e "\n=========================================="
echo "测试完成！"
echo "=========================================="
