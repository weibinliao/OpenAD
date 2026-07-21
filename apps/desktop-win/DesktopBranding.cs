using System.Drawing;
using System.Drawing.Drawing2D;
using System.Drawing.Imaging;
using System.Runtime.InteropServices;

namespace PermissionProtector.Desktop;

internal static class DesktopBranding
{
    private const int RuntimeIconSize = 256;
    private const int MaximumRuntimeLogoBytes = 512 * 1024;

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool DestroyIcon(IntPtr iconHandle);

    internal static Icon CreateApplicationIcon(string executablePath)
    {
        using var embeddedIcon = Icon.ExtractAssociatedIcon(executablePath)
            ?? throw new InvalidOperationException($"No application icon is embedded in {executablePath}.");
        return (Icon)embeddedIcon.Clone();
    }

    internal static void ApplyWindowIcon(Form window, string executablePath)
    {
        ArgumentNullException.ThrowIfNull(window);
        window.Icon = CreateApplicationIcon(executablePath);
    }

    internal static bool TryCreateRuntimeIcon(string dataUrl, out Icon? icon)
    {
        icon = null;
        if (string.IsNullOrWhiteSpace(dataUrl))
        {
            return false;
        }

        var separatorIndex = dataUrl.IndexOf(',');
        if (separatorIndex <= 0)
        {
            return false;
        }

        var header = dataUrl[..separatorIndex];
        if (header is not "data:image/png;base64" and not "data:image/jpeg;base64")
        {
            return false;
        }

        try
        {
            var bytes = Convert.FromBase64String(dataUrl[(separatorIndex + 1)..]);
            if (bytes.Length == 0 || bytes.Length > MaximumRuntimeLogoBytes)
            {
                return false;
            }

            using var stream = new MemoryStream(bytes, writable: false);
            using var source = Image.FromStream(stream, useEmbeddedColorManagement: true, validateImageData: true);
            using var canvas = new Bitmap(RuntimeIconSize, RuntimeIconSize, PixelFormat.Format32bppArgb);
            using (var graphics = Graphics.FromImage(canvas))
            {
                graphics.Clear(Color.Transparent);
                graphics.CompositingMode = CompositingMode.SourceOver;
                graphics.CompositingQuality = CompositingQuality.HighQuality;
                graphics.InterpolationMode = InterpolationMode.HighQualityBicubic;
                graphics.PixelOffsetMode = PixelOffsetMode.HighQuality;
                graphics.SmoothingMode = SmoothingMode.HighQuality;

                const int padding = 16;
                var available = RuntimeIconSize - (padding * 2);
                var scale = Math.Min((double)available / source.Width, (double)available / source.Height);
                var width = Math.Max(1, (int)Math.Round(source.Width * scale));
                var height = Math.Max(1, (int)Math.Round(source.Height * scale));
                var left = (RuntimeIconSize - width) / 2;
                var top = (RuntimeIconSize - height) / 2;
                graphics.DrawImage(source, new Rectangle(left, top, width, height));
            }

            var handle = canvas.GetHicon();
            try
            {
                using var handleIcon = Icon.FromHandle(handle);
                icon = (Icon)handleIcon.Clone();
                return true;
            }
            finally
            {
                DestroyIcon(handle);
            }
        }
        catch (ArgumentException)
        {
            return false;
        }
        catch (FormatException)
        {
            return false;
        }
        catch (OutOfMemoryException)
        {
            return false;
        }
    }
}
