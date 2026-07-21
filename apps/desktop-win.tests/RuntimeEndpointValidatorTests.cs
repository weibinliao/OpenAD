using Xunit;

namespace PermissionProtector.Desktop.Tests;

public sealed class RuntimeEndpointValidatorTests
{
    [Fact]
    public void AcceptsOnlyOpenAdApiHealth()
    {
        const string valid = """{"service":"openad","status":"healthy","database":true}""";
        const string generic = """{"status":"healthy","database":true}""";

        Assert.True(RuntimeEndpointValidator.IsApiHealth(valid));
        Assert.False(RuntimeEndpointValidator.IsApiHealth(generic));
    }

    [Fact]
    public void AcceptsOnlyOpenAdDesktopWebMarker()
    {
        const string valid = """{"product":"OpenAD","runtime":"desktop-web"}""";
        const string generic = """{"product":"OtherApp","runtime":"desktop-web"}""";

        Assert.True(RuntimeEndpointValidator.IsWebMarker(valid));
        Assert.False(RuntimeEndpointValidator.IsWebMarker(generic));
        Assert.False(RuntimeEndpointValidator.IsWebMarker("<html>Other app</html>"));
    }

    [Theory]
    [InlineData(18080, "API")]
    [InlineData(43110, "desktop web")]
    public void ConflictMessagesNameTheRoleAndPort(int port, string role)
    {
        var message = RuntimeEndpointValidator.PortConflictMessage(port, role);

        Assert.Contains(port.ToString(), message);
        Assert.Contains(role, message, StringComparison.OrdinalIgnoreCase);
    }
}
