namespace PermissionProtector.Desktop;

internal enum StartupExperiencePhase
{
    Preparing,
    Api,
    Workspace,
    Window,
    Complete,
    Failed,
}

internal sealed record StartupExperienceState(
    StartupExperiencePhase Phase,
    string Headline,
    string Detail,
    double TargetProgress,
    bool IsFailure)
{
    internal static StartupExperienceState FromRuntimeMessage(string runtimeMessage)
    {
        var message = runtimeMessage?.Trim() ?? string.Empty;

        if (message.StartsWith("Startup failed:", StringComparison.OrdinalIgnoreCase) ||
            message.StartsWith("Page load failed:", StringComparison.OrdinalIgnoreCase))
        {
            if (message.Contains("Desktop package is incomplete", StringComparison.OrdinalIgnoreCase))
            {
                return new(
                    StartupExperiencePhase.Failed,
                    "桌面组件不完整",
                    "请从完整发布目录启动 OpenAD；目录中应包含本地服务程序和 web 资源。",
                    0.82d,
                    true);
            }

            if (message.Contains("port 18080", StringComparison.OrdinalIgnoreCase) ||
                message.Contains("port 43110", StringComparison.OrdinalIgnoreCase))
            {
                return new(
                    StartupExperiencePhase.Failed,
                    "本地端口被占用",
                    "OpenAD 无法启动本地服务，请关闭占用端口的程序后重试。",
                    0.82d,
                    true);
            }

            if (message.StartsWith("Page load failed:", StringComparison.OrdinalIgnoreCase))
            {
                return new(
                    StartupExperiencePhase.Failed,
                    "工作台连接失败",
                    "OpenAD 本地服务已启动，但桌面界面未能完成加载，请重新启动。",
                    0.90d,
                    true);
            }

            return new(
                StartupExperiencePhase.Failed,
                "OpenAD 启动未完成",
                "请检查桌面组件和本地服务后重试。",
                0.82d,
                true);
        }

        if (message.StartsWith("Starting local API", StringComparison.OrdinalIgnoreCase))
        {
            return new(
                StartupExperiencePhase.Api,
                "正在启动权限服务",
                "建立仅限本机访问的 NTFS 与 AD 分析通道",
                0.24d,
                false);
        }

        if (message.Equals("API is ready.", StringComparison.OrdinalIgnoreCase))
        {
            return new(
                StartupExperiencePhase.Api,
                "权限服务已就绪",
                "本地 API 与审计数据存储已通过健康检查",
                0.46d,
                false);
        }

        if (message.StartsWith("Starting desktop web", StringComparison.OrdinalIgnoreCase))
        {
            return new(
                StartupExperiencePhase.Workspace,
                "正在装载 OpenAD 工作台",
                "准备目录浏览、权限扫描和报告模块",
                0.62d,
                false);
        }

        if (message.Equals("Desktop web is ready.", StringComparison.OrdinalIgnoreCase))
        {
            return new(
                StartupExperiencePhase.Workspace,
                "OpenAD 工作台已就绪",
                "本地静态资源与桌面会话已经建立",
                0.78d,
                false);
        }

        if (message.Equals("Starting desktop window...", StringComparison.OrdinalIgnoreCase))
        {
            return new(
                StartupExperiencePhase.Window,
                "正在连接桌面界面",
                "同步品牌、窗口控制和本地运行状态",
                0.90d,
                false);
        }

        return new(
            StartupExperiencePhase.Preparing,
            "正在建立受保护的工作区",
            "校验桌面运行时与本地组件",
            0.08d,
            false);
    }

    internal static StartupExperienceState Completed() =>
        new(
            StartupExperiencePhase.Complete,
            "保护工作区已就绪",
            "正在进入 OpenAD 权限运维总览",
            1d,
            false);
}
