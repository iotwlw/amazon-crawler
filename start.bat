@echo off
chcp 65001 > nul
setlocal enabledelayedexpansion

:: ============================================================
:: amazon-crawler 启动脚本 (Windows)
:: ============================================================

echo.
echo ================================================
echo         Amazon Crawler 启动脚本
echo ================================================
echo.

:: 检查配置文件
if not exist "config.yaml" (
    if exist "config.yaml.example" (
        echo [信息] 未找到 config.yaml，正在从 config.yaml.example 创建...
        copy /Y "config.yaml.example" "config.yaml" > nul
        echo [提示] 已生成默认配置，请先编辑 config.yaml 后重新运行
    ) else (
        echo [错误] 配置文件 config.yaml 不存在，且缺少模板 config.yaml.example
    )
    pause
    exit /b 1
)

:: 检查可执行文件
if not exist "amazon-crawler.exe" (
    echo [信息] 可执行文件不存在，开始编译...
    echo.
    where go > nul 2>&1
    if errorlevel 1 (
        echo [错误] 未找到 go 命令，无法自动编译
        pause
        exit /b 1
    )
    go build -o amazon-crawler.exe
    if errorlevel 1 (
        echo [错误] 编译失败
        pause
        exit /b 1
    )
    echo [成功] 编译完成
    echo.
)

:: 启动程序
echo [启动] 正在启动 amazon-crawler...
echo.
echo 配置文件: config.yaml
echo.
echo 按 Ctrl+C 停止程序
echo ================================================
echo.

amazon-crawler.exe -c config.yaml

:: 程序结束后暂停
echo.
echo ================================================
echo 程序已结束
echo ================================================
pause
