@echo off
@setlocal enableextensions
@cd /d "%~dp0"
sc create "ProxyService" binPath= "%cd%\http_proxy.exe %cd%"
sc config "ProxyService" start=auto
sc start "ProxyService"
powercfg-energy