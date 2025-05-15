# PowerShell脚本，用于在Windows环境下构建和部署Gateway容器

# 颜色常量
$Red = "Red"
$Green = "Green"
$Yellow = "Yellow"
$White = "White"

# 获取脚本目录
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
# 获取项目根目录
$RootDir = Split-Path -Parent $ScriptDir

# 输出信息
Write-Host "Gateway 容器构建脚本" -ForegroundColor $Green
Write-Host "项目路径: $RootDir" -ForegroundColor $Yellow

# 切换到项目根目录
Set-Location -Path $RootDir

# 检查docker是否安装
try {
    $dockerVersion = docker --version
    Write-Host "已检测到Docker: $dockerVersion" -ForegroundColor $Green
}
catch {
    Write-Host "错误: Docker未安装或未加入PATH环境变量！" -ForegroundColor $Red
    Write-Host "请安装Docker Desktop: https://www.docker.com/products/docker-desktop" -ForegroundColor $Red
    exit 1
}

# 创建.env文件（如果不存在），并包含代理设置占位符
$EnvFile = Join-Path -Path $ScriptDir -ChildPath ".env"
if (-not (Test-Path $EnvFile)) {
    Write-Host "创建默认.env文件于 $EnvFile" -ForegroundColor $Yellow
    @'
# 日志级别 (例如: debug, info, warn, error)
LOG_LEVEL=info

# Admin 服务配置
ADMIN_MONGO_CONNECTION_STRING=mongodb://10.17.191.106:27017
ADMIN_DATABASE_NAME=gateway_admin_v2
ADMIN_SERVER_PORT=8080

# 网络代理设置 (如果你的构建环境需要通过代理访问互联网, 请取消注释并配置以下变量)
# HTTP_PROXY_URL=http://your-proxy-username:your-proxy-password@your-proxy-address:port
# HTTPS_PROXY_URL=http://your-proxy-username:your-proxy-password@your-proxy-address:port
# NO_PROXY_HOSTS=localhost,127.0.0.1,.your-internal-domain.com,192.168.0.0/16
'@ | Out-File -FilePath $EnvFile -Encoding utf8
    Write-Host "已创建.env文件，请根据需要修改配置 (特别是代理和Admin服务配置)" -ForegroundColor $Green
}

# 构建选项
$Rebuild = Read-Host "是否清理Docker缓存并重新构建所有镜像? (y/n)"
if ($Rebuild -eq 'y' -or $Rebuild -eq 'Y') {
    Write-Host "清理Docker缓存并重新构建..." -ForegroundColor $Yellow
    docker-compose -f "$ScriptDir\docker-compose.yml" build --no-cache
}
else {
    Write-Host "使用缓存构建..." -ForegroundColor $Yellow
    docker-compose -f "$ScriptDir\docker-compose.yml" build
}

# 部署选项
$Deploy = Read-Host "是否立即部署并启动容器? (y/n)"
if ($Deploy -eq 'y' -or $Deploy -eq 'Y') {
    Write-Host "启动服务..." -ForegroundColor $Yellow
    docker-compose -f "$ScriptDir\docker-compose.yml" up -d

    Write-Host "服务已启动！" -ForegroundColor $Green
    Write-Host "前端访问地址: http://localhost:3000" -ForegroundColor $Green
    Write-Host "后端API地址: http://localhost:8080" -ForegroundColor $Green

    Write-Host "容器状态:" -ForegroundColor $Yellow
    docker-compose -f "$ScriptDir\docker-compose.yml" ps
}
else {
    Write-Host "构建完成。使用以下命令启动服务:" -ForegroundColor $Yellow
    Write-Host "cd $ScriptDir; docker-compose up -d" -ForegroundColor $Green
}

Write-Host "构建过程完成!" -ForegroundColor $Green


