using System.Globalization;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace PermissionProtector.Desktop;

internal enum StartupExperienceLocale
{
    English,
    SimplifiedChinese,
}

internal static class StartupLocaleStore
{
    internal const string FileName = "startup-locale.json";

    internal static StartupExperienceLocale Resolve(
        string dataDirectory,
        CultureInfo? fallbackCulture = null)
    {
        try
        {
            var path = Path.Combine(dataDirectory, FileName);
            if (File.Exists(path))
            {
                var document = JsonSerializer.Deserialize<StartupLocaleDocument>(
                    File.ReadAllText(path));
                if (TryParse(document?.Locale, out var savedLocale))
                {
                    return savedLocale;
                }
            }
        }
        catch (JsonException)
        {
        }
        catch (IOException)
        {
        }
        catch (UnauthorizedAccessException)
        {
        }

        var culture = fallbackCulture ?? CultureInfo.CurrentUICulture;
        return culture.TwoLetterISOLanguageName.Equals("zh", StringComparison.OrdinalIgnoreCase)
            ? StartupExperienceLocale.SimplifiedChinese
            : StartupExperienceLocale.English;
    }

    internal static bool TryPersist(string dataDirectory, string? webLocale)
    {
        if (!TryParse(webLocale, out var locale))
        {
            return false;
        }

        try
        {
            Directory.CreateDirectory(dataDirectory);
            var content = JsonSerializer.Serialize(
                new StartupLocaleDocument(ToWebLocale(locale)));
            File.WriteAllText(Path.Combine(dataDirectory, FileName), content);
            return true;
        }
        catch (IOException)
        {
            return false;
        }
        catch (UnauthorizedAccessException)
        {
            return false;
        }
    }

    internal static string ToWebLocale(StartupExperienceLocale locale) =>
        locale == StartupExperienceLocale.SimplifiedChinese ? "zh-CN" : "en";

    private static bool TryParse(string? value, out StartupExperienceLocale locale)
    {
        switch (value)
        {
            case "zh-CN":
                locale = StartupExperienceLocale.SimplifiedChinese;
                return true;
            case "en":
                locale = StartupExperienceLocale.English;
                return true;
            default:
                locale = StartupExperienceLocale.English;
                return false;
        }
    }

    private sealed record StartupLocaleDocument(
        [property: JsonPropertyName("locale")] string Locale);
}
