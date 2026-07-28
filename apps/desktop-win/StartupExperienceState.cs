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
    internal static StartupExperienceState FromRuntimeMessage(
        string runtimeMessage,
        StartupExperienceLocale locale)
    {
        var message = runtimeMessage?.Trim() ?? string.Empty;
        var strings = StartupExperienceStrings.For(locale);

        if (message.StartsWith("Startup failed:", StringComparison.OrdinalIgnoreCase) ||
            message.StartsWith("Page load failed:", StringComparison.OrdinalIgnoreCase))
        {
            if (message.Contains("Desktop package is incomplete", StringComparison.OrdinalIgnoreCase))
            {
                return Create(StartupExperiencePhase.Failed, strings.PackageIncomplete, 0.82d, true);
            }

            if (message.Contains("port 18080", StringComparison.OrdinalIgnoreCase) ||
                message.Contains("port 43110", StringComparison.OrdinalIgnoreCase))
            {
                return Create(StartupExperiencePhase.Failed, strings.PortConflict, 0.82d, true);
            }

            if (message.StartsWith("Page load failed:", StringComparison.OrdinalIgnoreCase))
            {
                return Create(StartupExperiencePhase.Failed, strings.PageLoadFailed, 0.90d, true);
            }

            return Create(StartupExperiencePhase.Failed, strings.StartupFailed, 0.82d, true);
        }

        if (message.StartsWith("Starting local API", StringComparison.OrdinalIgnoreCase))
        {
            return Create(StartupExperiencePhase.Api, strings.ApiStarting, 0.24d);
        }

        if (message.Equals("API is ready.", StringComparison.OrdinalIgnoreCase))
        {
            return Create(StartupExperiencePhase.Api, strings.ApiReady, 0.46d);
        }

        if (message.StartsWith("Starting desktop web", StringComparison.OrdinalIgnoreCase))
        {
            return Create(StartupExperiencePhase.Workspace, strings.WorkspaceStarting, 0.62d);
        }

        if (message.Equals("Desktop web is ready.", StringComparison.OrdinalIgnoreCase))
        {
            return Create(StartupExperiencePhase.Workspace, strings.WorkspaceReady, 0.78d);
        }

        if (message.Equals("Starting desktop window...", StringComparison.OrdinalIgnoreCase))
        {
            return Create(StartupExperiencePhase.Window, strings.WindowStarting, 0.90d);
        }

        return Create(StartupExperiencePhase.Preparing, strings.Preparing, 0.08d);
    }

    internal static StartupExperienceState Completed(StartupExperienceLocale locale) =>
        Create(
            StartupExperiencePhase.Complete,
            StartupExperienceStrings.For(locale).Complete,
            1d);

    private static StartupExperienceState Create(
        StartupExperiencePhase phase,
        StartupStatusStrings strings,
        double targetProgress,
        bool isFailure = false) =>
        new(phase, strings.Headline, strings.Detail, targetProgress, isFailure);
}
