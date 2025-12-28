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
    echo [错误] 配置文件 config.yaml 不存在
    echo.
    echo 请先复制配置模板:
    echo   copy config.yaml.save config.yaml
    echo.
    echo 然后编辑 config.yaml 配置数据库连接等参数
    pause
    exit /b 1
)

:: 检查可执行文件
if not exist "amazon-crawler.exe" (
    echo [信息] 可执行文件不存在，开始编译...
    echo.
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
