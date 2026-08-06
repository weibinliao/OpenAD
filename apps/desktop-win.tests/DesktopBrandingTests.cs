using System.Drawing;
using System.Security.Cryptography;
using System.Windows.Forms;
using System.Xml.Linq;
using Xunit;

namespace PermissionProtector.Desktop.Tests;

public sealed class DesktopBrandingTests
{
    [Fact]
    public void DesktopProjectEmbedsOpenADApplicationIcon()
    {
        var projectDirectory = FindProjectDirectory();
        var projectPath = Path.Combine(projectDirectory, "OpenAD.Desktop.csproj");
        var project = XDocument.Load(projectPath);
        var applicationIcon = project
            .Descendants()
            .SingleOrDefault(element => element.Name.LocalName == "ApplicationIcon")
            ?.Value;

        Assert.Equal("OpenAD.ico", applicationIcon);

        var iconPath = Path.Combine(projectDirectory, applicationIcon!);
        Assert.True(File.Exists(iconPath), $"Missing desktop icon: {iconPath}");

        using var icon = new Icon(iconPath);
        Assert.True(icon.Width >= 32);
        Assert.True(icon.Height >= 32);
    }

    [Fact]
    public void DesktopPackageReadmeUsesOpenADAsPrimaryProductName()
    {
        var projectDirectory = FindProjectDirectory();
        var rootDirectory = Directory.GetParent(projectDirectory)?.Parent?.FullName
            ?? throw new DirectoryNotFoundException("Could not locate repository root.");
        var buildScriptPath = Path.Combine(rootDirectory, "scripts", "build-desktop-windows.ps1");
        var buildScript = File.ReadAllText(buildScriptPath);

        Assert.Contains("'OpenAD Windows Desktop'", buildScript, StringComparison.Ordinal);
        Assert.DoesNotContain("'PermissionProtector Windows Desktop'", buildScript, StringComparison.Ordinal);
        Assert.Contains("OpenAD-Windows-Desktop-v", buildScript, StringComparison.Ordinal);
        Assert.DoesNotContain("PermissionProtector-Windows-Desktop-v", buildScript, StringComparison.Ordinal);
        Assert.Contains("Double-click OpenAD.exe", buildScript, StringComparison.Ordinal);
        Assert.DoesNotContain("PermissionProtector.exe", buildScript, StringComparison.Ordinal);
        // The active data directory must be OpenAD. The legacy path may still appear, but only
        // in the sentence explaining the one-time automatic migration.
        Assert.Contains("%APPDATA%\\OpenAD", buildScript, StringComparison.Ordinal);
        Assert.Contains("migrated automatically", buildScript, StringComparison.Ordinal);
    }

    [Fact]
    public void StartupChromeUsesOpenAdWithoutACompatibilityRuntimeBrand()
    {
        var projectDirectory = FindProjectDirectory();
        var startupControlPath = Path.Combine(projectDirectory, "StartupExperienceControl.cs");
        var startupStringsPath = Path.Combine(projectDirectory, "StartupExperienceStrings.cs");
        var source = File.ReadAllText(startupControlPath) + File.ReadAllText(startupStringsPath);
        var english = StartupExperienceStrings.For(StartupExperienceLocale.English);
        var chinese = StartupExperienceStrings.For(StartupExperienceLocale.SimplifiedChinese);

        Assert.Equal("LOCAL RUNTIME", english.LocalRuntime);
        Assert.Equal("本地运行时", chinese.LocalRuntime);
        Assert.DoesNotContain("RuntimeProjectName", source, StringComparison.Ordinal);
        Assert.DoesNotContain("\"PermissionProtector", source, StringComparison.Ordinal);
    }

    [Fact]
    public void WindowsShellIntegrationUsesOpenAdPublicNames()
    {
        var projectDirectory = FindProjectDirectory();
        var rootDirectory = Directory.GetParent(projectDirectory)?.Parent?.FullName
            ?? throw new DirectoryNotFoundException("Could not locate repository root.");
        var lanAccessScript = File.ReadAllText(Path.Combine(rootDirectory, "scripts", "enable-lan-access.bat"));
        var installTaskScript = File.ReadAllText(Path.Combine(rootDirectory, "scripts", "install-startup-task.bat"));

        Assert.Contains("OpenAD Web", lanAccessScript, StringComparison.Ordinal);
        Assert.Contains("OpenAD API", lanAccessScript, StringComparison.Ordinal);
        Assert.DoesNotContain("firewall add rule name=\"PermissionProtector", lanAccessScript, StringComparison.Ordinal);
        Assert.Contains("set \"TASK_NAME=OpenAD\"", installTaskScript, StringComparison.Ordinal);
    }

    [Fact]
    public void AppliesEmbeddedExecutableIconToDesktopWindow()
    {
        var executablePath = Path.Combine(AppContext.BaseDirectory, "OpenAD.exe");
        Assert.True(File.Exists(executablePath), $"Missing desktop executable: {executablePath}");

        using var expectedIcon = Icon.ExtractAssociatedIcon(executablePath)
            ?? throw new InvalidOperationException("The desktop executable has no embedded icon.");
        using var form = new Form();

        DesktopBranding.ApplyWindowIcon(form, executablePath);

        var actualIcon = form.Icon
            ?? throw new InvalidOperationException("The desktop window icon was not assigned.");
        Assert.Equal(IconHash(expectedIcon), IconHash(actualIcon));
    }

    [Fact]
    public void CreatesRuntimeWindowIconFromUploadedPngDataUrl()
    {
        using var source = new Bitmap(64, 32);
        using (var graphics = Graphics.FromImage(source))
        {
            graphics.Clear(Color.MediumPurple);
        }
        using var stream = new MemoryStream();
        source.Save(stream, System.Drawing.Imaging.ImageFormat.Png);
        var dataUrl = $"data:image/png;base64,{Convert.ToBase64String(stream.ToArray())}";

        var created = DesktopBranding.TryCreateRuntimeIcon(dataUrl, out var icon);

        Assert.True(created);
        Assert.NotNull(icon);
        using (icon)
        {
            Assert.Equal(256, icon.Width);
            Assert.Equal(256, icon.Height);
        }
    }

    [Theory]
    [InlineData("")]
    [InlineData("data:image/svg+xml;base64,PHN2Zy8+")]
    [InlineData("data:image/png;base64,not-base64")]
    public void RejectsUnsupportedOrMalformedRuntimeLogo(string dataUrl)
    {
        Assert.False(DesktopBranding.TryCreateRuntimeIcon(dataUrl, out var icon));
        Assert.Null(icon);
    }

    private static string IconHash(Icon icon)
    {
        using var bitmap = icon.ToBitmap();
        using var stream = new MemoryStream();
        bitmap.Save(stream, System.Drawing.Imaging.ImageFormat.Png);
        return Convert.ToHexString(SHA256.HashData(stream.ToArray()));
    }

    private static string FindProjectDirectory()
    {
        for (var current = new DirectoryInfo(AppContext.BaseDirectory); current is not null; current = current.Parent)
        {
            var projectPath = Path.Combine(
                current.FullName,
                "apps",
                "desktop-win",
                "OpenAD.Desktop.csproj");
            if (File.Exists(projectPath))
            {
                return Path.GetDirectoryName(projectPath)!;
            }
        }

        throw new DirectoryNotFoundException("Could not locate apps\\desktop-win.");
    }
}
