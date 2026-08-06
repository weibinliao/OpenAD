using Xunit;

namespace PermissionProtector.Desktop.Tests;

public sealed class DesktopRuntimeCacheTests
{
    [Fact]
    public void UsesUniqueDesktopSessionUrlToAvoidRestoringStaleWebViewContent()
    {
        using var first = new DesktopRuntime();
        using var second = new DesktopRuntime();

        Assert.Equal("http", first.AppUrl.Scheme);
        Assert.Equal("127.0.0.1", first.AppUrl.Host);
        Assert.Equal(43110, first.AppUrl.Port);
        Assert.Equal("/", first.AppUrl.AbsolutePath);
        Assert.Contains("desktop_session=", first.AppUrl.Query, StringComparison.Ordinal);
        Assert.NotEqual(first.AppUrl, second.AppUrl);
    }

    [Fact]
    public void IsolatesWebViewCacheWhenBundledWebAssetsChange()
    {
        var fixture = Path.Combine(Path.GetTempPath(), "openad-web-cache-" + Guid.NewGuid().ToString("N"));
        var firstWebRoot = Path.Combine(fixture, "first", "web");
        var secondWebRoot = Path.Combine(fixture, "second", "web");

        try
        {
            Directory.CreateDirectory(firstWebRoot);
            Directory.CreateDirectory(secondWebRoot);
            File.WriteAllText(Path.Combine(firstWebRoot, "index.html"), "<html>first</html>");
            File.WriteAllText(Path.Combine(secondWebRoot, "index.html"), "<html>second</html>");

            var cacheRoot = Path.Combine(fixture, "cache");
            var first = DesktopRuntime.ResolveWebViewDataDirectory(cacheRoot, firstWebRoot);
            var second = DesktopRuntime.ResolveWebViewDataDirectory(cacheRoot, secondWebRoot);

            Assert.NotEqual(first, second);
            Assert.Equal("WebView2", Path.GetFileName(Path.GetDirectoryName(first)!));
            Assert.Equal(12, Path.GetFileName(first).Length);
        }
        finally
        {
            if (Directory.Exists(fixture))
            {
                Directory.Delete(fixture, recursive: true);
            }
        }
    }

    [Fact]
    public void MigratesLegacyDatabaseFilenameToOpenAD()
    {
        var fixture = Path.Combine(Path.GetTempPath(), "openad-db-migration-" + Guid.NewGuid().ToString("N"));
        Directory.CreateDirectory(fixture);
        try
        {
            var legacy = Path.Combine(fixture, "permission-protector.db");
            File.WriteAllText(legacy, "legacy-db");
            File.WriteAllText(legacy + "-wal", "legacy-wal");

            var resolved = DesktopRuntime.ResolveDatabasePath(fixture);

            Assert.Equal(Path.Combine(fixture, "OpenAD.db"), resolved);
            Assert.True(File.Exists(Path.Combine(fixture, "OpenAD.db")));
            Assert.True(File.Exists(Path.Combine(fixture, "OpenAD.db-wal")));
            Assert.False(File.Exists(legacy));
        }
        finally
        {
            if (Directory.Exists(fixture))
            {
                Directory.Delete(fixture, recursive: true);
            }
        }
    }
}
