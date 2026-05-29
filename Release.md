

1. 前端本地构建

- default 新版前端（默认）
构建项目
``` bash
cd web/default/
#使用bun或npm都可以

# 安装依赖
# npm install
# 安装依赖,无添加依赖项，则只安装一次即可，新增依赖项后，需要重新安装
bun install
# 启动本地服务
# bun run dev
# 构建项目
bun run build 
```
-- classic 经典版本（旧版）
同default，构建项目，若以后不使用可去除和不更新。

2. 后端代码部署

1）后端go源代码更新
下载git库下载zip包到本地
2）上传到服务器/services目录
3）对c6c-api目录进行备份
``` bash
cd services
cp -r c6c-api c6c-api-[日期时间]
```
4）解压zip包覆盖c6c-api目录
``` bash
cd services
# 建议先清空（清空前务必备份下）
# rm -rf ./c6c-api/*
unzip c6c-api-main.zip -d c6c-api
```

3. 上传前端代码到服务器
本地web/default/dist 上传到服务器 /services/c6c-api/web/default目录
同理，若classic也需部署，则将本地web/classic/dist 上传到服务器 /services/c6c-api/web/classic目录

4. 重新构建docker镜像服务
``` bash
cd services/c6c-api
docker compose up -d --build
```
