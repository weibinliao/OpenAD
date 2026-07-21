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
        var state = StartupExperienceState.FromRuntimeMessage(runtimeMessage);

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
        var state = StartupExperienceState.FromRuntimeMessage(runtimeMessage);

        Assert.Equal(StartupExperiencePhase.Failed, state.Phase);
        Assert.True(state.IsFailure);
        Assert.False(string.IsNullOrWhiteSpace(state.Detail));
    }

    [Fact]
    public void ExplainsAnIncompleteDesktopPackageWithoutExposingTheRawEnglishRuntimeError()
    {
        var state = StartupExperienceState.FromRuntimeMessage(
            "Startup failed: Desktop package is incomplete. Put PermissionProtector.exe beside permission-protector-server.exe, permission-protector-web.exe, and web\\index.html.");

        Assert.Equal("桌面组件不完整", state.Headline);
        Assert.Contains("完整发布目录", state.Detail, StringComparison.Ordinal);
        Assert.DoesNotContain("PermissionProtector.exe", state.Detail, StringComparison.Ordinal);
    }

    [Fact]
    public void CompletionStateReachesFullProgress()
    {
        var state = StartupExperienceState.Completed();

        Assert.Equal(StartupExperiencePhase.Complete, state.Phase);
        Assert.Equal(1d, state.TargetProgress);
        Assert.False(state.IsFailure);
    }

    [Fact]
    public void KeepsTheStartupExperienceVisibleLongEnoughToRead()
    {
        Assert.InRange(
            StartupExperienceControl.MinimumVisibleDuration,
            TimeSpan.FromMilliseconds(1600),
            TimeSpan.FromMilliseconds(2200));
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
        using var startup = new StartupExperienceControl
        {
            Size = new System.Drawing.Size(980, 680),
        };
        startup.UpdateRuntimeStatus("Startup failed: test failure");
        startup.PerformLayout();
        var retry = startup.Controls
            .OfType<Button>()
            .Single(button => button.AccessibleName == "重新启动 OpenAD");
        var layout = StartupExperienceControl.CalculateLayout(startup.Size, showRetry: true);

        Assert.Equal((int)layout.Retry.Top, retry.Top);
        Assert.Equal((int)layout.Retry.Bottom, retry.Bottom);
        Assert.True(layout.Status.Bottom <= retry.Top);
        Assert.True(retry.Bottom <= layout.Progress.Top);
    }

    [Fact]
    public void RendersAComposedFirstFrameInsteadOfAFlatBlackSurface()
    {
        using var startup = new StartupExperienceControl
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
            showRetry: true);

        Assert.True(layout.Brand.Bottom <= layout.Status.Top);
        Assert.True(layout.Status.Bottom <= layout.Retry.Top);
        Assert.True(layout.Retry.Bottom <= layout.Progress.Top);
        Assert.True(layout.Content.Width <= 760f);
    }
}
