using Xunit;

namespace PermissionProtector.Desktop.Tests;

public sealed class DesktopInstallerTests
{
    [Fact]
    public void InstallerUsesPerUserLocationAndDoesNotBundleApplicationData()
    {
        var root = FindRepositoryRoot();
        var installerScript = File.ReadAllText(Path.Combine(root, "installer", "OpenAD.iss"));

        Assert.Contains("DefaultDirName={localappdata}\\Programs\\OpenAD", installerScript, StringComparison.Ordinal);
        Assert.Contains("PrivilegesRequired=lowest", installerScript, StringComparison.Ordinal);
        Assert.Contains("OutputBaseFilename=OpenAD", installerScript, StringComparison.Ordinal);
        Assert.Contains("Source: \"{#SourceDir}\\*\"", installerScript, StringComparison.Ordinal);
        Assert.Contains("*.db", installerScript, StringComparison.Ordinal);
        Assert.Contains(".env", installerScript, StringComparison.Ordinal);
        Assert.DoesNotContain("{appdata}", installerScript, StringComparison.OrdinalIgnoreCase);
        Assert.DoesNotContain("PermissionProtector", installerScript, StringComparison.Ordinal);
    }

    [Fact]
    public void InstallerBuildUsesSelfContainedPrivacyAuditedPayload()
    {
        var root = FindRepositoryRoot();
        var installerBuild = File.ReadAllText(Path.Combine(root, "scripts", "build-windows-installer.ps1"));
        var desktopBuild = File.ReadAllText(Path.Combine(root, "scripts", "build-desktop-windows.ps1"));
        var packageAudit = File.ReadAllText(Path.Combine(root, "scripts", "audit-release-package.ps1"));

        Assert.Contains("-SelfContained", installerBuild, StringComparison.Ordinal);
        Assert.Contains("audit-release-package.ps1", installerBuild, StringComparison.Ordinal);
        Assert.Contains("-AdditionalArtifactPath $installerPath", installerBuild, StringComparison.Ordinal);
        Assert.Contains("$outputBaseName = 'OpenAD'", installerBuild, StringComparison.Ordinal);
        Assert.Contains("OpenAD-v$Version-win-x64", installerBuild, StringComparison.Ordinal);
        Assert.Contains("Compress-Archive", installerBuild, StringComparison.Ordinal);
        Assert.Contains("OpenAD.Server.exe", packageAudit, StringComparison.Ordinal);
        Assert.Contains("OpenAD.CLI.exe", packageAudit, StringComparison.Ordinal);
        Assert.Contains("OpenAD.Web.exe", packageAudit, StringComparison.Ordinal);
        Assert.DoesNotContain("permission-protector-server.exe", packageAudit, StringComparison.OrdinalIgnoreCase);
        Assert.Contains("-trimpath", desktopBuild, StringComparison.Ordinal);
        Assert.Contains("-buildvcs=false", desktopBuild, StringComparison.Ordinal);
        Assert.Contains("/p:DebugSymbols=false", desktopBuild, StringComparison.Ordinal);
        Assert.Contains("/p:DebugType=None", desktopBuild, StringComparison.Ordinal);
        Assert.Contains("*.pdb", packageAudit, StringComparison.Ordinal);
        Assert.Contains("QQ email address", packageAudit, StringComparison.Ordinal);
        Assert.Contains("RFC 1918 address literal", packageAudit, StringComparison.Ordinal);
        Assert.Contains("Windows user profile path", packageAudit, StringComparison.Ordinal);
    }

    [Fact]
    public void InstallerToolchainPinsCompilerAndChineseTranslationChecksums()
    {
        var root = FindRepositoryRoot();
        var setupScript = File.ReadAllText(Path.Combine(root, "scripts", "setup-inno-setup.ps1"));

        Assert.Contains("6.7.3", setupScript, StringComparison.Ordinal);
        Assert.Contains("ExpectedSha256", setupScript, StringComparison.Ordinal);
        Assert.Contains("ChineseLanguageSha256", setupScript, StringComparison.Ordinal);
        Assert.Contains("Installed Inno Setup language checksum mismatch", setupScript, StringComparison.Ordinal);
        Assert.Contains("Get-AuthenticodeSignature", setupScript, StringComparison.Ordinal);
    }

    private static string FindRepositoryRoot()
    {
        for (var current = new DirectoryInfo(AppContext.BaseDirectory); current is not null; current = current.Parent)
        {
            if (File.Exists(Path.Combine(current.FullName, "AGENTS.md"))
                && File.Exists(Path.Combine(current.FullName, "scripts", "build-desktop-windows.ps1")))
            {
                return current.FullName;
            }
        }

        throw new DirectoryNotFoundException("Could not locate the OpenAD repository root.");
    }
}
