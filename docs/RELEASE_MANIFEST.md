# OpenAD Windows 桌面包清单

[中文](#openad-windows-桌面包清单) | [English](#openad-windows-desktop-package-manifest)

Windows 桌面包由以下命令生成：

```powershell
.\scripts\build-desktop-windows.ps1
```

默认输出目录：

```text
dist\OpenAD-Windows-Desktop-v1.0.0
```

## 必需文件

| 文件或目录 | 用途 |
| --- | --- |
| `OpenAD.exe` | OpenAD 桌面启动程序 |
| `OpenAD.dll` 及运行时元数据 | .NET WinForms/WebView2 桌面宿主 |
| `OpenAD.Server.exe` | Go API 服务 |
| `OpenAD.Web.exe` | 内置静态 Web 服务 |
| `OpenAD.CLI.exe` | 本地 CLI |
| `web\` | Next.js 静态产品界面 |
| `Microsoft.Web.WebView2.*`、`WebView2Loader.dll` | WebView2 集成文件 |
| `runtimes\` | 平台相关原生资源 |
| `OpenAD-README.txt` | 启动和数据路径说明 |
| `start-windows.bat` | 浏览器模式备用启动脚本 |
| `stop-windows.bat` | 运行时停止脚本 |
| `verify-install.bat` | 备用安装包检查脚本 |

## 运行约定

- 用户可见产品名称：OpenAD
- API：`http://127.0.0.1:18080`
- 桌面包 Web：`http://127.0.0.1:43110`
- 本地数据：`%APPDATA%\OpenAD`

首次升级会把旧 `%APPDATA%\PermissionProtector` 数据目录迁移到 OpenAD；发布包文件名不再使用旧产品名。

## 验证

启动 `OpenAD.exe` 后运行：

```powershell
Invoke-RestMethod http://127.0.0.1:18080/health
Invoke-WebRequest http://127.0.0.1:43110/
```

同时确认：

- 窗口标题和可见品牌是 OpenAD。
- 启动过程不出现中间错误框，能够进入产品主界面。
- 现有 AD 连接仍能通过测试。
- 窗口可以从四条边和四个角缩放。

## OpenAD Windows Desktop Package Manifest

Build the Windows desktop package with:

```powershell
.\scripts\build-desktop-windows.ps1
```

Default output directory:

```text
dist\OpenAD-Windows-Desktop-v1.0.0
```

### Required Files

| File or directory | Purpose |
| --- | --- |
| `OpenAD.exe` | OpenAD desktop launcher |
| `OpenAD.dll` and runtime metadata | .NET WinForms/WebView2 desktop host |
| `OpenAD.Server.exe` | Go API service |
| `OpenAD.Web.exe` | Embedded static Web service |
| `OpenAD.CLI.exe` | Local CLI |
| `web\` | Next.js static product interface |
| `Microsoft.Web.WebView2.*`, `WebView2Loader.dll` | WebView2 integration files |
| `runtimes\` | Platform-specific native resources |
| `OpenAD-README.txt` | Startup and data-path guidance |
| `start-windows.bat` | Browser-mode fallback launcher |
| `stop-windows.bat` | Runtime shutdown script |
| `verify-install.bat` | Fallback package verification script |

### Runtime Contract

- User-visible product name: OpenAD
- API: `http://127.0.0.1:18080`
- Packaged Web: `http://127.0.0.1:43110`
- Local data: `%APPDATA%\OpenAD`

First upgrade migrates the legacy `%APPDATA%\PermissionProtector` data directory to OpenAD; release package filenames no longer use the legacy product name.

### Verification

After starting `OpenAD.exe`, run:

```powershell
Invoke-RestMethod http://127.0.0.1:18080/health
Invoke-WebRequest http://127.0.0.1:43110/
```

Also verify that the visible brand and window title use OpenAD, startup reaches the product without an intermediate error frame, the existing AD connection still passes, and the window resizes from all four edges and four corners.
