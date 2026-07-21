using System.Diagnostics;
using System.Net;
using System.Net.Http;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Text;

namespace PermissionProtector.Desktop;

internal sealed class DesktopRuntime : IDisposable
{
    private const int ApiPort = 18080;
    private const int WebPort = 43110;
    private static readonly Uri ApiHealthUrl = new($"http://127.0.0.1:{ApiPort}/health");
    private static readonly Uri WebMarkerUrl = new($"http://127.0.0.1:{WebPort}/openad-runtime.json");
    private readonly HttpClient http = new(new SocketsHttpHandler { UseProxy = false }) { Timeout = TimeSpan.FromSeconds(2) };
    private readonly List<Process> started = [];

    public string DataDirectory { get; } = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData), "PermissionProtector");
    public string WebViewDataDirectory { get; private set; } = string.Empty;
    public Uri AppUrl { get; } = new($"http://127.0.0.1:{WebPort}/?desktop_session={Guid.NewGuid():N}");

    public async Task StartAsync(Action<string> status, CancellationToken token)
    {
        var root = FindRuntimeRoot();
        var webRoot = Path.Combine(root, "web");
        WebViewDataDirectory = ResolveWebViewDataDirectory(
            Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "PermissionProtector"),
            webRoot);
        Directory.CreateDirectory(DataDirectory);
        Directory.CreateDirectory(WebViewDataDirectory);
        var api = Path.Combine(root, "permission-protector-server.exe");
        var web = Path.Combine(root, "permission-protector-web.exe");

        var apiState = await ProbeAsync(ApiHealthUrl, ApiPort, RuntimeEndpointValidator.IsApiHealth, token);
        Process? apiProcess = null;
        if (apiState == EndpointState.Ready)
        {
            status("API is ready.");
        }
        else
        {
            EnsurePortAvailable(apiState, ApiPort, "API");
            status($"Starting local API on 127.0.0.1:{ApiPort}...");
            var db = Path.Combine(DataDirectory, "permission-protector.db");
            apiProcess = Start(root, api, "", new Dictionary<string, string>
            {
                ["API_HOST"] = "127.0.0.1",
                ["API_PORT"] = ApiPort.ToString(),
                ["PORT"] = ApiPort.ToString(),
                ["GIN_MODE"] = "release",
                ["PERMISSION_PROTECTOR_DATA_DIR"] = DataDirectory,
                ["DATABASE_URL"] = "sqlite://" + db
            });
            await WaitFor(
                () => ProbeAsync(ApiHealthUrl, ApiPort, RuntimeEndpointValidator.IsApiHealth, token),
                "API",
                ApiPort,
                apiProcess,
                status,
                token);
        }

        var webState = await ProbeAsync(WebMarkerUrl, WebPort, RuntimeEndpointValidator.IsWebMarker, token);
        Process? webProcess = null;
        if (webState == EndpointState.Ready)
        {
            status("Desktop web is ready.");
        }
        else
        {
            EnsurePortAvailable(webState, WebPort, "desktop web");
            status($"Starting desktop web on 127.0.0.1:{WebPort}...");
            webProcess = Start(root, web, $"-root {Quote(webRoot)} -port {WebPort} -host 127.0.0.1", new Dictionary<string, string>());
            await WaitFor(
                () => ProbeAsync(WebMarkerUrl, WebPort, RuntimeEndpointValidator.IsWebMarker, token),
                "Desktop web",
                WebPort,
                webProcess,
                status,
                token);
        }
    }

    internal static string ResolveWebViewDataDirectory(string cacheRoot, string webRoot)
    {
        if (string.IsNullOrWhiteSpace(cacheRoot))
        {
            throw new ArgumentException("A WebView cache root is required.", nameof(cacheRoot));
        }
        if (!Directory.Exists(webRoot))
        {
            throw new DirectoryNotFoundException("Bundled web assets are missing: " + webRoot);
        }

        return Path.Combine(cacheRoot, "WebView2", CalculateWebAssetFingerprint(webRoot));
    }

    private static string CalculateWebAssetFingerprint(string webRoot)
    {
        using var fingerprint = IncrementalHash.CreateHash(HashAlgorithmName.SHA256);
        foreach (var assetPath in Directory.EnumerateFiles(webRoot, "*", SearchOption.AllDirectories)
                     .OrderBy(path => path, StringComparer.OrdinalIgnoreCase))
        {
            var relativePath = Path.GetRelativePath(webRoot, assetPath).Replace('\\', '/');
            fingerprint.AppendData(Encoding.UTF8.GetBytes(relativePath));
            fingerprint.AppendData([0]);
            using var asset = File.OpenRead(assetPath);
            fingerprint.AppendData(SHA256.HashData(asset));
        }

        return Convert.ToHexString(fingerprint.GetHashAndReset())[..12].ToLowerInvariant();
    }

    public void Dispose()
    {
        foreach (var process in started)
        {
            try
            {
                if (!process.HasExited)
                {
                    process.Kill(entireProcessTree: true);
                }
            }
            catch
            {
            }
            process.Dispose();
        }
        http.Dispose();
    }

    private static string FindRuntimeRoot()
    {
        var roots = new[]
        {
            Environment.GetEnvironmentVariable("PERMISSION_PROTECTOR_RUNTIME_DIR"),
            AppContext.BaseDirectory,
            Environment.CurrentDirectory
        };

        foreach (var root in roots.Where(value => !string.IsNullOrWhiteSpace(value)).Select(value => Path.GetFullPath(value!)))
        {
            if (File.Exists(Path.Combine(root, "permission-protector-server.exe"))
                && File.Exists(Path.Combine(root, "permission-protector-web.exe"))
                && File.Exists(Path.Combine(root, "web", "index.html")))
            {
                return root;
            }
        }

        throw new InvalidOperationException("Desktop package is incomplete. Keep the OpenAD launcher (PermissionProtector.exe), permission-protector-server.exe, permission-protector-web.exe, and web\\index.html together.");
    }

    private Process Start(string root, string fileName, string arguments, IReadOnlyDictionary<string, string> environment)
    {
        var info = new ProcessStartInfo(fileName, arguments)
        {
            WorkingDirectory = root,
            UseShellExecute = false,
            CreateNoWindow = true
        };
        foreach (var item in environment)
        {
            info.Environment[item.Key] = item.Value;
        }
        var process = Process.Start(info) ?? throw new InvalidOperationException("Failed to start " + fileName);
        started.Add(process);
        return process;
    }

    private static void EnsurePortAvailable(EndpointState state, int port, string role)
    {
        if (state == EndpointState.Bound)
        {
            throw new InvalidOperationException(RuntimeEndpointValidator.PortConflictMessage(port, role));
        }
    }

    private async Task WaitFor(
        Func<Task<EndpointState>> probe,
        string name,
        int port,
        Process ownedProcess,
        Action<string> status,
        CancellationToken token)
    {
        var end = DateTimeOffset.UtcNow.AddSeconds(25);
        while (DateTimeOffset.UtcNow < end)
        {
            if (await probe() == EndpointState.Ready)
            {
                status(name + " is ready.");
                return;
            }
            if (ownedProcess.HasExited)
            {
                throw new InvalidOperationException(Path.GetFileName(ownedProcess.StartInfo.FileName) + " exited with code " + ownedProcess.ExitCode);
            }
            await Task.Delay(500, token);
        }
        throw new TimeoutException($"{name} did not become ready on 127.0.0.1:{port}.");
    }

    private async Task<EndpointState> ProbeAsync(
        Uri url,
        int port,
        Func<string, bool> validator,
        CancellationToken token)
    {
        try
        {
            using var response = await http.GetAsync(url, token);
            var payload = await response.Content.ReadAsStringAsync(token);
            return response.IsSuccessStatusCode && validator(payload)
                ? EndpointState.Ready
                : EndpointState.Bound;
        }
        catch (OperationCanceledException) when (!token.IsCancellationRequested)
        {
            return await IsPortBoundAsync(port, token) ? EndpointState.Bound : EndpointState.Free;
        }
        catch (HttpRequestException)
        {
            return await IsPortBoundAsync(port, token) ? EndpointState.Bound : EndpointState.Free;
        }
    }

    private static async Task<bool> IsPortBoundAsync(int port, CancellationToken token)
    {
        using var client = new TcpClient();
        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(token);
        timeout.CancelAfter(TimeSpan.FromMilliseconds(350));
        try
        {
            await client.ConnectAsync(IPAddress.Loopback, port, timeout.Token);
            return client.Connected;
        }
        catch (OperationCanceledException) when (!token.IsCancellationRequested)
        {
            return false;
        }
        catch (SocketException)
        {
            return false;
        }
    }

    private static string Quote(string value) => "\"" + value.Replace("\"", "\\\"", StringComparison.Ordinal) + "\"";

    private enum EndpointState
    {
        Free,
        Bound,
        Ready
    }
}
