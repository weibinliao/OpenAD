using System.Net;
using System.Net.Http;
using Xunit;

namespace PermissionProtector.Desktop.Tests;

public sealed class DesktopUpdateServiceTests
{
    [Theory]
    [InlineData("1.0.0", "1.0.1", true)]
    [InlineData("v1.0.0", "1.0.0", false)]
    [InlineData("1.2.0", "1.1.9", false)]
    [InlineData("1.0.0+build.12", "v1.0.1", true)]
    public void ComparesReleaseVersions(string current, string candidate, bool expected)
    {
        Assert.Equal(expected, DesktopUpdateService.IsNewerVersion(current, candidate));
    }

    [Fact]
    public void NormalizesInformationalVersions()
    {
        Assert.Equal("1.2.3", DesktopUpdateService.NormalizeVersion("v1.2.3+commit.abc"));
    }

    [Fact]
    public void AcceptsOnlyTheOfficialReleaseAssetPath()
    {
        var valid = new Uri("https://github.com/weibinliao/OpenAD/releases/download/v1.0.1/OpenAD.exe");
        var otherRepository = new Uri("https://github.com/other/OpenAD/releases/download/v1.0.1/OpenAD.exe");
        var otherAsset = new Uri("https://github.com/weibinliao/OpenAD/releases/download/v1.0.1/other.exe");
        var query = new Uri("https://github.com/weibinliao/OpenAD/releases/download/v1.0.1/OpenAD.exe?download=1");

        Assert.True(DesktopUpdateService.IsAllowedReleaseAssetUri(valid, "v1.0.1", "OpenAD.exe"));
        Assert.False(DesktopUpdateService.IsAllowedReleaseAssetUri(otherRepository, "v1.0.1", "OpenAD.exe"));
        Assert.False(DesktopUpdateService.IsAllowedReleaseAssetUri(otherAsset, "v1.0.1", "OpenAD.exe"));
        Assert.False(DesktopUpdateService.IsAllowedReleaseAssetUri(query, "v1.0.1", "OpenAD.exe"));
    }

    [Theory]
    [InlineData("39ca83c3a962659b740508b343356be1796fd2cc74cfb8944eaf8348473aa452 *OpenAD.exe")]
    [InlineData("39CA83C3A962659B740508B343356BE1796FD2CC74CFB8944EAF8348473AA452  OpenAD.exe")]
    public void ParsesInstallerChecksum(string content)
    {
        Assert.Equal(
            "39ca83c3a962659b740508b343356be1796fd2cc74cfb8944eaf8348473aa452",
            DesktopUpdateService.ParseChecksum(content));
    }

    [Fact]
    public void RejectsChecksumForAnotherFile()
    {
        Assert.Throws<InvalidOperationException>(() =>
            DesktopUpdateService.ParseChecksum(
                "39ca83c3a962659b740508b343356be1796fd2cc74cfb8944eaf8348473aa452 *other.exe"));
    }

    [Fact]
    public async Task VerifiesDownloadedInstallerChecksum()
    {
        var path = Path.Combine(Path.GetTempPath(), "openad-update-hash-" + Guid.NewGuid().ToString("N") + ".exe");
        try
        {
            await File.WriteAllTextAsync(path, "verified installer fixture");
            var expected = Convert.ToHexString(System.Security.Cryptography.SHA256.HashData(
                await File.ReadAllBytesAsync(path))).ToLowerInvariant();

            await DesktopUpdateService.VerifySha256Async(path, expected, CancellationToken.None);
            await Assert.ThrowsAsync<InvalidOperationException>(() =>
                DesktopUpdateService.VerifySha256Async(path, new string('0', 64), CancellationToken.None));
        }
        finally
        {
            if (File.Exists(path))
            {
                File.Delete(path);
            }
        }
    }

    [Fact]
    public async Task ChecksTheLatestReleaseAndReportsAvailableUpdate()
    {
        using var http = new HttpClient(new StubHandler(_ => new HttpResponseMessage(HttpStatusCode.OK)
        {
            Content = new StringContent("""
                {
                  "tag_name": "v1.0.1",
                  "name": "OpenAD v1.0.1",
                  "body": "Bug fixes",
                  "html_url": "https://github.com/weibinliao/OpenAD/releases/tag/v1.0.1",
                  "published_at": "2026-08-07T00:00:00Z",
                  "draft": false,
                  "prerelease": false,
                  "assets": [
                    { "name": "OpenAD.exe", "browser_download_url": "https://github.com/weibinliao/OpenAD/releases/download/v1.0.1/OpenAD.exe" },
                    { "name": "OpenAD.exe.sha256", "browser_download_url": "https://github.com/weibinliao/OpenAD/releases/download/v1.0.1/OpenAD.exe.sha256" }
                  ]
                }
                """)
        }));
        using var service = new DesktopUpdateService(http, "1.0.0");

        var result = await service.CheckAsync(CancellationToken.None);

        Assert.True(result.UpdateAvailable);
        Assert.Equal("1.0.0", result.CurrentVersion);
        Assert.Equal("1.0.1", result.LatestVersion);
        Assert.Equal("Bug fixes", result.ReleaseNotes);
    }

    [Fact]
    public async Task RejectsReleaseWithoutTheTrustedInstaller()
    {
        using var http = new HttpClient(new StubHandler(_ => new HttpResponseMessage(HttpStatusCode.OK)
        {
            Content = new StringContent("""
                { "tag_name": "v1.0.1", "assets": [], "draft": false, "prerelease": false }
                """)
        }));
        using var service = new DesktopUpdateService(http, "1.0.0");

        await Assert.ThrowsAsync<InvalidOperationException>(() => service.CheckAsync(CancellationToken.None));
    }

    private sealed class StubHandler(Func<HttpRequestMessage, HttpResponseMessage> handler) : HttpMessageHandler
    {
        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            return Task.FromResult(handler(request));
        }
    }
}
