namespace PermissionProtector.Desktop;

internal static class Program
{
    [STAThread]
    private static int Main(string[] args)
    {
        if (args.Any(argument => string.Equals(argument, "--smoke", StringComparison.OrdinalIgnoreCase)))
        {
            return RunSmokeCheck();
        }

        using var singleInstance = SingleInstanceLock.TryAcquire();
        if (singleInstance is null)
        {
            SingleInstanceActivation.RequestExistingWindow();
            return 0;
        }

        ApplicationConfiguration.Initialize();
        Application.Run(new MainForm());
        return 0;
    }

    private static int RunSmokeCheck()
    {
        try
        {
            using var runtime = new DesktopRuntime();
            runtime.StartAsync(_ => { }, CancellationToken.None).GetAwaiter().GetResult();
            return 0;
        }
        catch (Exception exception)
        {
            File.WriteAllText(Path.Combine(Path.GetTempPath(), "OpenAD.Desktop.Smoke.err.log"), exception.ToString());
            return 1;
        }
    }
}
