using System.Text.Json;

namespace PermissionProtector.Desktop;

internal static class RuntimeEndpointValidator
{
    public static bool IsApiHealth(string payload) => Matches(payload, root =>
        root.TryGetProperty("service", out var service)
        && string.Equals(service.GetString(), "openad", StringComparison.Ordinal)
        && root.TryGetProperty("status", out var status)
        && string.Equals(status.GetString(), "healthy", StringComparison.Ordinal)
        && root.TryGetProperty("database", out var database)
        && database.ValueKind == JsonValueKind.True);

    public static bool IsWebMarker(string payload) => Matches(payload, root =>
        root.TryGetProperty("product", out var product)
        && string.Equals(product.GetString(), "OpenAD", StringComparison.Ordinal)
        && root.TryGetProperty("runtime", out var runtime)
        && string.Equals(runtime.GetString(), "desktop-web", StringComparison.Ordinal));

    public static string PortConflictMessage(int port, string role) =>
        $"Port {port} is already used by another application, so the {role} service cannot start. "
        + "Close the program using this port, then select Retry.";

    private static bool Matches(string payload, Func<JsonElement, bool> predicate)
    {
        try
        {
            using var document = JsonDocument.Parse(payload);
            return predicate(document.RootElement);
        }
        catch (JsonException)
        {
            return false;
        }
    }
}
