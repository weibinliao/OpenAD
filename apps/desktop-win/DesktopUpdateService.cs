using System.Diagnostics;
using System.Net.Http;
using System.Reflection;
using System.Security.Cryptography;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Text.RegularExpressions;

namespace PermissionProtector.Desktop;

internal sealed class DesktopUpdateService : IDisposable
{
    internal const string RepositoryOwner = "weibinliao";
    internal const string RepositoryName = "OpenAD";
    internal const string InstallerAssetName = "OpenAD.exe";
    internal const string ChecksumAssetName = "OpenAD.exe.sha256";
    internal static readonly Uri LatestReleaseApiUri = new(
        $"https://api.github.com/repos/{RepositoryOwner}/{RepositoryName}/releases/latest");
    private static readonly Regex Sha256Pattern = new("^[0-9a-f]{64}$", RegexOptions.IgnoreCase | RegexOptions.CultureInvariant);
    private static readonly JsonSerializerOptions JsonOptions = new() { PropertyNameCaseInsensitive = true };
    private readonly HttpClient http;
    private readonly bool disposeHttpClient;
    private readonly string currentVersion;

    public DesktopUpdateService()
        : this(CreateHttpClient(), currentVersion: null, disposeHttpClient: true)
    {
    }

    internal DesktopUpdateService(HttpClient http, string? currentVersion = null, bool disposeHttpClient = false)
    {
        this.http = http ?? throw new ArgumentNullException(nameof(http));
        this.currentVersion = NormalizeVersion(currentVersion ?? ResolveCurrentVersion());
        this.disposeHttpClient = disposeHttpClient;
    }

    internal string CurrentVersion => currentVersion;

    internal async Task<DesktopUpdateCheckResult> CheckAsync(CancellationToken token)
    {
        using var response = await http.GetAsync(LatestReleaseApiUri, token).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();
        var payload = await response.Content.ReadAsStringAsync(token).ConfigureAwait(false);
        var release = JsonSerializer.Deserialize<GitHubRelease>(payload, JsonOptions)
            ?? throw new InvalidOperationException("GitHub returned an empty release response.");

        if (release.Draft || release.Prerelease)
        {
            throw new InvalidOperationException("The latest OpenAD release is not available for installation yet.");
        }

        var latestVersion = NormalizeVersion(release.TagName);
        var installer = FindAndValidateAsset(release, InstallerAssetName);
        var checksum = FindAndValidateAsset(release, ChecksumAssetName);
        var releaseUrl = ValidateReleasePageUri(release.HtmlUrl, release.TagName);
        var installerUrl = new Uri(installer.BrowserDownloadUrl, UriKind.Absolute);
        var checksumUrl = new Uri(checksum.BrowserDownloadUrl, UriKind.Absolute);

        return new DesktopUpdateCheckResult(
            currentVersion,
            latestVersion,
            release.Name?.Trim() ?? $"OpenAD v{latestVersion}",
            TrimReleaseNotes(release.Body),
            release.PublishedAt,
            releaseUrl,
            installerUrl,
            checksumUrl,
            IsNewerVersion(currentVersion, latestVersion));
    }

    internal async Task<DesktopUpdateInstallResult> DownloadAndLaunchAsync(CancellationToken token)
    {
        var update = await CheckAsync(token).ConfigureAwait(false);
        if (!update.UpdateAvailable)
        {
            return new DesktopUpdateInstallResult(false, update.LatestVersion, null);
        }

        var tempPath = Path.Combine(
            Path.GetTempPath(),
            $"OpenAD-update-{update.LatestVersion}-{Guid.NewGuid():N}.exe");
        var launched = false;
        try
        {
            await DownloadFileAsync(update.InstallerUrl, tempPath, token).ConfigureAwait(false);
            var expectedHash = await DownloadChecksumAsync(update.ChecksumUrl, token).ConfigureAwait(false);
            await VerifySha256Async(tempPath, expectedHash, token).ConfigureAwait(false);

            var process = Process.Start(new ProcessStartInfo
            {
                FileName = tempPath,
                WorkingDirectory = Path.GetDirectoryName(tempPath) ?? Path.GetTempPath(),
                UseShellExecute = true,
                CreateNoWindow = false,
            });
            if (process is null)
            {
                throw new InvalidOperationException("Windows could not start the OpenAD installer.");
            }

            launched = true;
            return new DesktopUpdateInstallResult(true, update.LatestVersion, tempPath);
        }
        catch
        {
            if (!launched)
            {
                TryDelete(tempPath);
            }
            throw;
        }
    }

    internal static bool IsNewerVersion(string current, string candidate)
    {
        return Version.TryParse(NormalizeVersion(current), out var currentVersion)
            && Version.TryParse(NormalizeVersion(candidate), out var candidateVersion)
            && candidateVersion > currentVersion;
    }

    internal static string NormalizeVersion(string value)
    {
        var normalized = (value ?? string.Empty).Trim();
        if (normalized.StartsWith('v'))
        {
            normalized = normalized[1..];
        }

        var separator = normalized.IndexOf('+');
        if (separator >= 0)
        {
            normalized = normalized[..separator];
        }

        if (!Version.TryParse(normalized, out var version))
        {
            throw new InvalidOperationException($"OpenAD version '{value}' is invalid.");
        }

        return $"{version.Major}.{version.Minor}.{Math.Max(0, version.Build)}";
    }

    internal static bool IsAllowedReleaseAssetUri(Uri uri, string tagName, string assetName)
    {
        if (uri.Scheme != Uri.UriSchemeHttps ||
            !string.Equals(uri.Host, "github.com", StringComparison.OrdinalIgnoreCase))
        {
            return false;
        }

        var expectedPath = $"/{RepositoryOwner}/{RepositoryName}/releases/download/{Uri.EscapeDataString(tagName)}/{assetName}";
        return string.Equals(uri.AbsolutePath, expectedPath, StringComparison.OrdinalIgnoreCase)
            && string.IsNullOrEmpty(uri.Query)
            && string.IsNullOrEmpty(uri.Fragment);
    }

    internal static string ParseChecksum(string content)
    {
        var tokens = content
            .Split((char[]?)null, StringSplitOptions.RemoveEmptyEntries)
            .Select(token => token.Trim())
            .ToArray();
        if (tokens.Length < 2 || !Sha256Pattern.IsMatch(tokens[0]) ||
            !string.Equals(tokens[1].TrimStart('*'), InstallerAssetName, StringComparison.Ordinal))
        {
            throw new InvalidOperationException("The OpenAD installer checksum file is invalid.");
        }

        return tokens[0].ToLowerInvariant();
    }

    private static HttpClient CreateHttpClient()
    {
        var client = new HttpClient(new SocketsHttpHandler { UseProxy = true })
        {
            Timeout = TimeSpan.FromMinutes(10),
        };
        client.DefaultRequestHeaders.UserAgent.ParseAdd("OpenAD-Updater/1.0");
        client.DefaultRequestHeaders.Accept.ParseAdd("application/vnd.github+json");
        return client;
    }

    private static string ResolveCurrentVersion()
    {
        var informational = typeof(DesktopUpdateService).Assembly
            .GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion;
        return string.IsNullOrWhiteSpace(informational)
            ? typeof(DesktopUpdateService).Assembly.GetName().Version?.ToString(3) ?? "0.0.0"
            : informational;
    }

    private static GitHubReleaseAsset FindAndValidateAsset(GitHubRelease release, string assetName)
    {
        var asset = release.Assets.FirstOrDefault(item =>
            string.Equals(item.Name, assetName, StringComparison.Ordinal));
        if (asset is null || !Uri.TryCreate(asset.BrowserDownloadUrl, UriKind.Absolute, out var uri) ||
            !IsAllowedReleaseAssetUri(uri, release.TagName, assetName))
        {
            throw new InvalidOperationException($"GitHub release is missing the trusted {assetName} asset.");
        }

        return asset;
    }

    private static Uri ValidateReleasePageUri(string? htmlUrl, string tagName)
    {
        if (Uri.TryCreate(htmlUrl, UriKind.Absolute, out var uri) &&
            uri.Scheme == Uri.UriSchemeHttps &&
            string.Equals(uri.Host, "github.com", StringComparison.OrdinalIgnoreCase) &&
            string.Equals(uri.AbsolutePath, $"/{RepositoryOwner}/{RepositoryName}/releases/tag/{Uri.EscapeDataString(tagName)}", StringComparison.OrdinalIgnoreCase))
        {
            return uri;
        }

        return new Uri($"https://github.com/{RepositoryOwner}/{RepositoryName}/releases/tag/{Uri.EscapeDataString(tagName)}");
    }

    private async Task<string> DownloadChecksumAsync(Uri checksumUri, CancellationToken token)
    {
        using var response = await http.GetAsync(checksumUri, token).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();
        return ParseChecksum(await response.Content.ReadAsStringAsync(token).ConfigureAwait(false));
    }

    private async Task DownloadFileAsync(Uri uri, string destination, CancellationToken token)
    {
        using var response = await http.GetAsync(uri, HttpCompletionOption.ResponseHeadersRead, token).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();
        await using var output = new FileStream(destination, FileMode.CreateNew, FileAccess.Write, FileShare.None, 128 * 1024, useAsync: true);
        await response.Content.CopyToAsync(output, token).ConfigureAwait(false);
    }

    internal static async Task VerifySha256Async(string path, string expectedHash, CancellationToken token)
    {
        await using var input = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read, 128 * 1024, useAsync: true);
        using var sha256 = SHA256.Create();
        var actualHash = Convert.ToHexString(await sha256.ComputeHashAsync(input, token).ConfigureAwait(false)).ToLowerInvariant();
        if (!CryptographicOperations.FixedTimeEquals(
                Convert.FromHexString(actualHash),
                Convert.FromHexString(expectedHash)))
        {
            throw new InvalidOperationException("The downloaded OpenAD installer failed SHA-256 verification.");
        }
    }

    private static string TrimReleaseNotes(string? body)
    {
        var notes = (body ?? string.Empty).Trim();
        return notes.Length <= 12000 ? notes : notes[..12000].TrimEnd() + "\n…";
    }

    private static void TryDelete(string path)
    {
        try
        {
            if (File.Exists(path))
            {
                File.Delete(path);
            }
        }
        catch
        {
            // A partial download is temporary and can be cleaned up by the OS later.
        }
    }

    public void Dispose()
    {
        if (disposeHttpClient)
        {
            http.Dispose();
        }
    }

    internal sealed record DesktopUpdateCheckResult(
        string CurrentVersion,
        string LatestVersion,
        string ReleaseName,
        string ReleaseNotes,
        DateTimeOffset? PublishedAt,
        Uri ReleaseUrl,
        Uri InstallerUrl,
        Uri ChecksumUrl,
        bool UpdateAvailable);

    internal sealed record DesktopUpdateInstallResult(bool Launched, string Version, string? InstallerPath);

    private sealed class GitHubRelease
    {
        [JsonPropertyName("tag_name")]
        public string TagName { get; set; } = string.Empty;

        [JsonPropertyName("name")]
        public string? Name { get; set; }

        [JsonPropertyName("body")]
        public string? Body { get; set; }

        [JsonPropertyName("html_url")]
        public string? HtmlUrl { get; set; }

        [JsonPropertyName("published_at")]
        public DateTimeOffset? PublishedAt { get; set; }

        [JsonPropertyName("draft")]
        public bool Draft { get; set; }

        [JsonPropertyName("prerelease")]
        public bool Prerelease { get; set; }

        [JsonPropertyName("assets")]
        public List<GitHubReleaseAsset> Assets { get; set; } = [];
    }

    private sealed class GitHubReleaseAsset
    {
        [JsonPropertyName("name")]
        public string Name { get; set; } = string.Empty;

        [JsonPropertyName("browser_download_url")]
        public string BrowserDownloadUrl { get; set; } = string.Empty;
    }
}
