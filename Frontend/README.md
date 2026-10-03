# SuperBizAgent 前端使用指南

## 功能介绍

SuperBizAgent 前端提供了以下功能：

### 1. 智能对话
- **普通对话模式**：传统的请求-响应模式
- **流式对话模式**：实时流式响应，类似 ChatGPT

### 2. 文件上传到知识库 ✨ 新功能
- 支持点击上传和拖拽上传两种方式
- 支持的文件格式：PDF, TXT, MD, CSV, DOC, DOCX
- 文件大小限制：50MB
- 上传后文件会自动保存到知识库

## 使用方法

### 启动前端服务

```bash
cd SuperBizAgentFrontend
chmod +x start.sh
./start.sh
```

或者直接用浏览器打开 `index.html` 文件。

### 上传文件到知识库

有两种方式上传文件：

#### 方式1: 点击上传
1. 点击"上传文件到知识库"区域
2. 在弹出的文件选择器中选择文件
3. 等待上传完成

#### 方式2: 拖拽上传
1. 直接将文件拖拽到"上传文件到知识库"区域
2. 松开鼠标完成上传
3. 等待上传完成

### 上传成功后
- 会显示绿色的成功提示
- 显示上传的文件名和文件大小
- 文件会保存到 `/Users/wuxufei/GolandProjects/SuperBizAgent/docs/` 目录

## 后端 API

### 分片断点上传与合并接口

1. **秒传与进度检查**：`POST /api/upload/check`
   - 参数：`fileMd5`, `fileName`, `totalChunks`
   - 响应：`isUploaded`（秒传成功）或 `uploadedChunks`（已完成分片索引）

2. **分片上传**：`POST /api/upload/chunk`
   - 参数：`file`（分片数据）, `fileMd5`, `chunkIndex`
   - 响应：分片上传状态

3. **分片合并**：`POST /api/upload/merge`
   - 参数：`fileMd5`, `fileName`, `totalChunks`, `totalSize`
   - 响应：合并完成并由 Kafka 异步触发 Tika 抽取与向量化

4. **知识库文档管理**：
   - 列表查询：`GET /api/documents/list`
   - 在线预览：`GET /api/documents/preview?fileMd5=...`
   - 安全下载：`GET /api/documents/download?fileMd5=...`
   - 级联删除：`DELETE /api/documents/:fileMd5`

## 技术栈

- 原生 JavaScript (ES6+)
- Fetch API
- FormData API
- Drag and Drop API
- Server-Sent Events (SSE)

## 浏览器兼容性

- Chrome/Edge 90+
- Firefox 88+
- Safari 14+

## 注意事项

1. 确保后端服务已启动（默认端口：6872）
2. 上传大文件时请耐心等待
3. 如果上传失败，请检查：
   - 文件大小是否超过50MB
   - 文件格式是否支持
   - 后端服务是否正常运行
   - 保存目录是否有写入权限
