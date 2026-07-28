namespace PermissionProtector.Desktop;

internal sealed record StartupStatusStrings(string Headline, string Detail);

internal sealed record StartupExperienceStrings(
    string AccessibleName,
    string Subtitle,
    string MinimizeAccessibleName,
    string CloseAccessibleName,
    string RetryText,
    string RetryAccessibleName,
    string OpenSourceLocalFirst,
    string LocalRuntime,
    string[] StageLabels,
    StartupStatusStrings Preparing,
    StartupStatusStrings ApiStarting,
    StartupStatusStrings ApiReady,
    StartupStatusStrings WorkspaceStarting,
    StartupStatusStrings WorkspaceReady,
    StartupStatusStrings WindowStarting,
    StartupStatusStrings Complete,
    StartupStatusStrings PackageIncomplete,
    StartupStatusStrings PortConflict,
    StartupStatusStrings PageLoadFailed,
    StartupStatusStrings StartupFailed)
{
    private static readonly StartupExperienceStrings Chinese = new(
        "OpenAD 启动",
        "开放的 Active Directory 权限洞察与运维工作台",
        "最小化窗口",
        "关闭窗口",
        "重新启动",
        "重新启动 OpenAD",
        "开源 · 本地优先",
        "本地运行时",
        ["运行时", "权限服务", "工作台", "桌面界面"],
        new("正在建立受保护的工作区", "校验桌面运行时与本地组件"),
        new("正在启动权限服务", "建立仅限本机访问的 NTFS 与 AD 分析通道"),
        new("权限服务已就绪", "本地 API 与审计数据存储已通过健康检查"),
        new("正在装载 OpenAD 工作台", "准备目录浏览、权限扫描和报告模块"),
        new("OpenAD 工作台已就绪", "本地静态资源与桌面会话已经建立"),
        new("正在连接桌面界面", "同步品牌、窗口控制和本地运行状态"),
        new("保护工作区已就绪", "正在进入 OpenAD 权限运维总览"),
        new("桌面组件不完整", "请从完整发布目录启动 OpenAD；目录中应包含本地服务程序和 web 资源。"),
        new("本地端口被占用", "OpenAD 无法启动本地服务，请关闭占用端口的程序后重试。"),
        new("工作台连接失败", "OpenAD 本地服务已启动，但桌面界面未能完成加载，请重新启动。"),
        new("OpenAD 启动未完成", "请检查桌面组件和本地服务后重试。"));

    private static readonly StartupExperienceStrings English = new(
        "OpenAD startup",
        "Open Active Directory permission insights and operations workspace",
        "Minimize window",
        "Close window",
        "Restart",
        "Restart OpenAD",
        "OPEN SOURCE · LOCAL FIRST",
        "LOCAL RUNTIME",
        ["Runtime", "Permission service", "Workspace", "Desktop"],
        new("Preparing the protected workspace", "Validating the desktop runtime and local components"),
        new("Starting the permission service", "Establishing local-only NTFS and AD analysis channels"),
        new("Permission service is ready", "The local API and audit data store passed health checks"),
        new("Loading the OpenAD workspace", "Preparing directory, permission scan, and report modules"),
        new("OpenAD workspace is ready", "Local assets and the desktop session are ready"),
        new("Connecting the desktop interface", "Synchronizing branding, window controls, and runtime status"),
        new("Protected workspace is ready", "Opening the OpenAD permission operations overview"),
        new("Desktop components are incomplete", "Start OpenAD from the complete release folder containing the local services and web assets."),
        new("Local port is already in use", "OpenAD cannot start its local services. Close the program using the port and try again."),
        new("Workspace connection failed", "OpenAD started its local services, but the desktop interface could not load. Restart OpenAD."),
        new("OpenAD startup did not complete", "Check the desktop components and local services, then try again."));

    internal static StartupExperienceStrings For(StartupExperienceLocale locale) =>
        locale == StartupExperienceLocale.SimplifiedChinese ? Chinese : English;
}
