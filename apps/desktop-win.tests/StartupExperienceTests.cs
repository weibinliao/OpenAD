using System.Globalization;
using System.Text.Json;
using System.Windows.Forms;
using Microsoft.Web.WebView2.WinForms;
using Xunit;

namespace PermissionProtector.Desktop.Tests;

public sealed class StartupExperienceTests
{
    [Theory]
    [InlineData("Preparing desktop runtime...", "Preparing", 0.08)]
    [InlineData("Starting local API on 127.0.0.1:18080...", "Api", 0.24)]
    [InlineData("API is ready.", "Api", 0.46)]
    [InlineData("Starting desktop web on 127.0.0.1:43110...", "Workspace", 0.62)]
    [InlineData("Desktop web is ready.", "Workspace", 0.78)]
    [InlineData("Starting desktop window...", "Window", 0.9)]
    public void MapsRuntimeMessagesToMeaningfulStartupStages(
        string runtimeMessage,
        string expectedPhase,
        double expectedProgress)
    {
        var state = StartupExperienceState.FromRuntimeMessage(
            runtimeMessage,
            StartupExperienceLocale.SimplifiedChinese);

        Assert.Equal(expectedPhase, state.Phase.ToString());
        Assert.Equal(expectedProgress, state.TargetProgress, precision: 2);
        Assert.False(state.IsFailure);
        Assert.False(string.IsNullOrWhiteSpace(state.Headline));
        Assert.False(string.IsNullOrWhiteSpace(state.Detail));
    }

    [Theory]
    [InlineData("Startup failed: port 18080 is busy.")]
    [InlineData("Page load failed: ConnectionAborted")]
    public void PreservesActionableFailureDetails(string runtimeMessage)
    {
        var state = StartupExperienceState.FromRuntimeMessage(
            runtimeMessage,
            StartupExperienceLocale.SimplifiedChinese);

        Assert.Equal(StartupExperiencePhase.Failed, state.Phase);
        Assert.True(state.IsFailure);
        Assert.False(string.IsNullOrWhiteSpace(state.Detail));
    }

    [Fact]
    public void ExplainsAnIncompleteDesktopPackageWithoutExposingTheRawEnglishRuntimeError()
    {
        var state = StartupExperienceState.FromRuntimeMessage(
            "Startup failed: Desktop package is incomplete. Put PermissionProtector.exe beside permission-protector-server.exe, permission-protector-web.exe, and web\\index.html.",
            StartupExperienceLocale.SimplifiedChinese);

        Assert.Equal("桌面组件不完整", state.Headline);
        Assert.Contains("完整发布目录", state.Detail, StringComparison.Ordinal);
        Assert.DoesNotContain("PermissionProtector.exe", state.Detail, StringComparison.Ordinal);
    }

    [Fact]
    public void ExplainsAnIncompleteDesktopPackageInEnglishWithoutExposingInternalFileNames()
    {
        var state = StartupExperienceState.FromRuntimeMessage(
            "Startup failed: Desktop package is incomplete. Put PermissionProtector.exe beside permission-protector-server.exe, permission-protector-web.exe, and web\\index.html.",
            StartupExperienceLocale.English);

        Assert.Equal("Desktop components are incomplete", state.Headline);
        Assert.Contains("complete release folder", state.Detail, StringComparison.OrdinalIgnoreCase);
        Assert.DoesNotContain("PermissionProtector.exe", state.Detail, StringComparison.Ordinal);
        Assert.DoesNotContain("permission-protector-server.exe", state.Detail, StringComparison.Ordinal);
    }

    [Theory]
    [InlineData("Preparing desktop runtime...", "Preparing the protected workspace")]
    [InlineData("Starting local API on 127.0.0.1:18080...", "Starting the permission service")]
    [InlineData("API is ready.", "Permission service is ready")]
    [InlineData("Starting desktop web on 127.0.0.1:43110...", "Loading the OpenAD workspace")]
    [InlineData("Desktop web is ready.", "OpenAD workspace is ready")]
    [InlineData("Starting desktop window...", "Connecting the desktop interface")]
    public void MapsRuntimeMessagesToEnglishPresentation(
        string runtimeMessage,
        string expectedHeadline)
    {
        var state = StartupExperienceState.FromRuntimeMessage(
            runtimeMessage,
            StartupExperienceLocale.English);

        Assert.Equal(expectedHeadline, state.Headline);
        Assert.False(string.IsNullOrWhiteSpace(state.Detail));
    }

    [Theory]
    [InlineData("Startup failed: port 18080 is busy.", "zh-CN", "本地端口被占用")]
    [InlineData("Page load failed: ConnectionAborted", "zh-CN", "工作台连接失败")]
    [InlineData("Startup failed: unexpected", "zh-CN", "OpenAD 启动未完成")]
    [InlineData("Startup failed: port 18080 is busy.", "en", "Local port is already in use")]
    [InlineData("Page load failed: ConnectionAborted", "en", "Workspace connection failed")]
    [InlineData("Startup failed: unexpected", "en", "OpenAD startup did not complete")]
    public void LocalizesEveryFailureCategory(
        string runtimeMessage,
        string webLocale,
        string expectedHeadline)
    {
        var locale = webLocale == "zh-CN"
            ? StartupExperienceLocale.SimplifiedChinese
            : StartupExperienceLocale.English;

        var state = StartupExperienceState.FromRuntimeMessage(runtimeMessage, locale);

        Assert.Equal(expectedHeadline, state.Headline);
        Assert.True(state.IsFailure);
    }

    [Fact]
    public void ProvidesLocalizedChromeAndStageLabels()
    {
        var chinese = StartupExperienceStrings.For(StartupExperienceLocale.SimplifiedChinese);
        var english = StartupExperienceStrings.For(StartupExperienceLocale.English);

        Assert.Equal(["运行时", "权限服务", "工作台", "桌面界面"], chinese.StageLabels);
        Assert.Equal(["Runtime", "Permission service", "Workspace", "Desktop"], english.StageLabels);
        Assert.Contains("Active Directory", chinese.Subtitle, StringComparison.Ordinal);
        Assert.Contains("Active Directory", english.Subtitle, StringComparison.Ordinal);
    }

    [Fact]
    public void CompletionStateReachesFullProgress()
    {
        var state = StartupExperienceState.Completed(StartupExperienceLocale.SimplifiedChinese);

        Assert.Equal(StartupExperiencePhase.Complete, state.Phase);
        Assert.Equal(1d, state.TargetProgress);
        Assert.False(state.IsFailure);
    }

    [Fact]
    public void KeepsTheStartupExperienceVisibleLongEnoughToRead()
    {
        Assert.InRange(
            StartupExperienceControl.MinimumVisibleDuration,
            TimeSpan.FromMilliseconds(600),
            TimeSpan.FromMilliseconds(900));
        Assert.InRange(
            StartupExperienceControl.FadeOutDuration,
            TimeSpan.FromMilliseconds(180),
            TimeSpan.FromMilliseconds(320));
    }

    [Fact]
    public async Task DoesNotCompleteOrFadeARecoverableFailure()
    {
        using var startup = new StartupExperienceControl(StartupExperienceLocale.English);
        startup.UpdateRuntimeStatus("Startup failed: test failure");

        var completed = await startup.CompleteAsync();

        Assert.False(completed);
        Assert.Contains(
            startup.Controls.OfType<Button>(),
            button => button.AccessibleName == "Restart OpenAD" && button.Visible);
    }

    [Fact]
    public void UsesEnglishAccessibleActionsForAnEnglishStartupExperience()
    {
        using var startup = new StartupExperienceControl(StartupExperienceLocale.English);

        Assert.Contains(startup.Controls.OfType<Button>(), button => button.AccessibleName == "Minimize window");
        Assert.Contains(startup.Controls.OfType<Button>(), button => button.AccessibleName == "Close window");
        Assert.Contains(startup.Controls.OfType<Button>(), button => button.AccessibleName == "Restart OpenAD");
    }

    [Fact]
    public void ExposesAnAssemblyVersionForStartupDiagnostics()
    {
        Assert.StartsWith("v", StartupExperienceControl.ApplicationVersionLabel, StringComparison.Ordinal);
        Assert.Matches("^v\\d+\\.\\d+\\.\\d+", StartupExperienceControl.ApplicationVersionLabel);
    }

    [Fact]
    public void MainFormUsesTheDedicatedStartupExperienceInsteadOfALegacyCard()
    {
        using var form = new MainForm();
        var startup = form.Controls.OfType<StartupExperienceControl>().Single();

        Assert.Equal("OpenAD", form.Text);
        Assert.Equal(DockStyle.Fill, startup.Dock);
        Assert.Same(form, startup.Parent);
        Assert.DoesNotContain(
            form.Controls.OfType<Panel>(),
            panel => panel.BackColor == System.Drawing.Color.FromArgb(9, 9, 11));
    }

    [Fact]
    public void KeepsWebViewHiddenUntilTheStartupExperienceCompletes()
    {
        using var form = new MainForm();
        var webView = form.Controls.OfType<WebView2>().Single();
        var startup = form.Controls.OfType<StartupExperienceControl>().Single();

        form.Controls.Remove(webView);
        form.Controls.Remove(startup);
        Assert.False(webView.Visible);
        Assert.True(startup.Visible);

        webView.Dispose();
        startup.Dispose();
    }

    [Fact]
    public void KeepsRetryActionClearOfStatusAndProgressAtMinimumWindowSize()
    {
        using var startup = new StartupExperienceControl(StartupExperienceLocale.SimplifiedChinese)
        {
            Size = new System.Drawing.Size(980, 680),
        };
        startup.UpdateRuntimeStatus("Startup failed: test failure");
        startup.PerformLayout();
        var retry = startup.Controls
            .OfType<Button>()
            .Single(button => button.AccessibleName == "重新启动 OpenAD");
        var layout = StartupExperienceControl.CalculateLayout(startup.Size, showRetry: true, deviceDpi: 96);

        Assert.Equal((int)layout.Retry.Top, retry.Top);
        Assert.Equal((int)layout.Retry.Bottom, retry.Bottom);
        Assert.True(layout.Status.Bottom <= retry.Top);
        Assert.True(retry.Bottom <= layout.Progress.Top);
    }

    [Fact]
    public void RendersAComposedFirstFrameInsteadOfAFlatBlackSurface()
    {
        using var startup = new StartupExperienceControl(StartupExperienceLocale.SimplifiedChinese)
        {
            Size = new System.Drawing.Size(1280, 860),
        };
        startup.CreateControl();
        using var preview = new System.Drawing.Bitmap(startup.Width, startup.Height);

        startup.DrawToBitmap(preview, startup.ClientRectangle);

        var sampledColors = new HashSet<int>();
        for (var y = 0; y < preview.Height; y += 43)
        {
            for (var x = 0; x < preview.Width; x += 53)
            {
                sampledColors.Add(preview.GetPixel(x, y).ToArgb());
            }
        }

        Assert.True(sampledColors.Count >= 8);
        Assert.DoesNotContain(System.Drawing.Color.Black.ToArgb(), sampledColors);
    }

    [Fact]
    public void UsesOpenAdAsThePrimaryStartupBrand()
    {
        Assert.Equal("OpenAD", StartupExperienceControl.PrimaryProductName);
    }

    [Theory]
    [InlineData(1280, 860)]
    [InlineData(980, 680)]
    public void KeepsBrandStatusRetryAndProgressInSeparateVerticalRegions(int width, int height)
    {
        var layout = StartupExperienceControl.CalculateLayout(
            new System.Drawing.Size(width, height),
            showRetry: true,
            deviceDpi: 96);

        Assert.True(layout.Brand.Bottom <= layout.Status.Top);
        Assert.True(layout.Status.Bottom <= layout.Retry.Top);
        Assert.True(layout.Retry.Bottom <= layout.Progress.Top);
        Assert.True(layout.Content.Width <= 760f);
    }

    [Fact]
    public void ScalesStartupLayoutAndChromeMetricsAtOneHundredFiftyPercentDpi()
    {
        var layoutAt96 = StartupExperienceControl.CalculateLayout(
            new System.Drawing.Size(1280, 860),
            showRetry: true,
            deviceDpi: 96);
        var layoutAt144 = StartupExperienceControl.CalculateLayout(
            new System.Drawing.Size(1920, 1290),
            showRetry: true,
            deviceDpi: 144);

        Assert.Equal(layoutAt96.Brand.X * 1.5f, layoutAt144.Brand.X, precision: 2);
        Assert.Equal(layoutAt96.Brand.Y * 1.5f, layoutAt144.Brand.Y, precision: 2);
        Assert.Equal(layoutAt96.Brand.Width * 1.5f, layoutAt144.Brand.Width, precision: 2);
        Assert.Equal(layoutAt96.Brand.Height * 1.5f, layoutAt144.Brand.Height, precision: 2);
        Assert.Equal(layoutAt96.Mark.Width * 1.5f, layoutAt144.Mark.Width, precision: 2);
        Assert.Equal(layoutAt96.Status.Height * 1.5f, layoutAt144.Status.Height, precision: 2);
        Assert.Equal(layoutAt96.Retry.Width * 1.5f, layoutAt144.Retry.Width, precision: 2);
        Assert.Equal(layoutAt96.Progress.Height * 1.5f, layoutAt144.Progress.Height, precision: 2);
        Assert.Equal(72f, StartupExperienceControl.TitleBarHeightForDpi(144));
    }

    [Theory]
    [InlineData("zh-CN", "SimplifiedChinese")]
    [InlineData("en", "English")]
    public void LoadsAWhitelistedSavedStartupLocale(
        string savedLocale,
        string expectedLocale)
    {
        using var directory = new TemporaryDirectory();
        File.WriteAllText(
            Path.Combine(directory.Path, StartupLocaleStore.FileName),
            JsonSerializer.Serialize(new { locale = savedLocale }));

        var locale = StartupLocaleStore.Resolve(directory.Path, new CultureInfo("fr-FR"));

        Assert.Equal(expectedLocale, locale.ToString());
    }

    [Theory]
    [InlineData("zh-CN", "SimplifiedChinese")]
    [InlineData("zh-TW", "SimplifiedChinese")]
    [InlineData("en-US", "English")]
    [InlineData("fr-FR", "English")]
    public void FallsBackToWindowsCultureWhenNoSavedLocaleExists(
        string cultureName,
        string expectedLocale)
    {
        using var directory = new TemporaryDirectory();

        var locale = StartupLocaleStore.Resolve(directory.Path, new CultureInfo(cultureName));

        Assert.Equal(expectedLocale, locale.ToString());
    }

    [Theory]
    [InlineData("not-json")]
    [InlineData("{\"locale\":\"zh-TW\"}")]
    [InlineData("{\"locale\":42}")]
    public void FallsBackToWindowsCultureForDamagedOrUnsupportedSavedLocales(string content)
    {
        using var directory = new TemporaryDirectory();
        File.WriteAllText(Path.Combine(directory.Path, StartupLocaleStore.FileName), content);

        var locale = StartupLocaleStore.Resolve(directory.Path, new CultureInfo("en-US"));

        Assert.Equal(StartupExperienceLocale.English, locale);
    }

    [Fact]
    public void PersistsOnlyWhitelistedWebLocales()
    {
        using var directory = new TemporaryDirectory();

        Assert.True(StartupLocaleStore.TryPersist(directory.Path, "en"));
        Assert.False(StartupLocaleStore.TryPersist(directory.Path, "zh-TW"));

        using var document = JsonDocument.Parse(
            File.ReadAllText(Path.Combine(directory.Path, StartupLocaleStore.FileName)));
        Assert.Equal("en", document.RootElement.GetProperty("locale").GetString());
    }

    private sealed class TemporaryDirectory : IDisposable
    {
        internal TemporaryDirectory()
        {
            Path = System.IO.Path.Combine(
                System.IO.Path.GetTempPath(),
                "OpenAD.StartupExperienceTests",
                Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(Path);
        }

        internal string Path { get; }

        public void Dispose()
        {
            if (Directory.Exists(Path))
            {
                Directory.Delete(Path, recursive: true);
            }
        }
    }
}
