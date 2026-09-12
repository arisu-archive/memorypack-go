using MemoryPack;
using System.Text.Json;

// This executable is the independent wire-format oracle for the Go tests.
var fixtures = new SortedDictionary<string, string>();
void Add<T>(string name, T value, MemoryPackSerializerOptions? options = null)
{
    var bytes = MemoryPackSerializer.Serialize(value, options);
    fixtures.Add(name, Convert.ToHexString(bytes));
    // Verify that the reference decoder accepts the fixture as its declared schema.
    var decoded = MemoryPackSerializer.Deserialize<T>(bytes);
    if (!bytes.AsSpan().SequenceEqual(MemoryPackSerializer.Serialize(decoded, options)))
        throw new Exception($"Reference round trip differs for {name}");
}

Add("int8-negative", (sbyte)-128);
Add("uint8", byte.MaxValue);
Add("uint16", ushort.MaxValue);
Add("uint32", uint.MaxValue);
Add("uint64", ulong.MaxValue);
Add("utf8", "A界\U0001D11E");
Add("utf16", "A界\U0001D11E", MemoryPackSerializerOptions.Utf16);
Add<string?>("string-null", null);
Add("string-empty", "");
Add<bool?>("nullable-bool", true);
Add<byte?>("nullable-byte", 255);
Add<short?>("nullable-short", -255);
Add<int?>("nullable-int", 255);
Add<uint?>("nullable-uint", uint.MaxValue);
Add<long?>("nullable-long", -1);
Add<ulong?>("nullable-ulong", ulong.MaxValue);
Add<float?>("nullable-float", 1.5f);
Add<double?>("nullable-double", -2.25);
Add<int?>("nullable-int-null", null);
Add<long?>("nullable-long-null", null);
Add<IMessage>("union-small", new NumberMessage { Value = 42 });
Add<IMessage>("union-249", new BoundaryMessage { Value = -1 });
Add<IMessage>("union-wide", new TextMessage { Value = "hello" });
Add<IMessage>("union-max", new MaxMessage { Value = 65535 });
Add<IMessage?>("union-null", null);
Add("union-list", new IMessage?[] { new NumberMessage { Value = 42 }, null, new TextMessage { Value = "hello" } });
Add("union-holder", new Envelope { Message = new TextMessage { Value = "hello" } });
Add("union-map", new Dictionary<string, IMessage> { ["item"] = new NumberMessage { Value = 42 } });
Add("object-old", new OldObject { Value = 42 });
Add("object-new", new NewObject { Value = 42, Text = "hello" });
Add("version-old", new OldVersion { Value = 42, Removed = 999, Text = "hello" });
Add("version-new", new NewVersion { Value = 42, Text = "hello", Added = 7 });
Add("version-long", new LongVersion { Text = new string('a', 120) });
Add("nullable-holder", new NullableHolder { Number = 255, Text = null });

var evolved = MemoryPackSerializer.Deserialize<NewVersion>(Convert.FromHexString(fixtures["version-old"]))!;
if (evolved.Value != 42 || evolved.Text != "hello" || evolved.Added != 0)
    throw new Exception("Old-to-new version-tolerant schema failed");
var previous = MemoryPackSerializer.Deserialize<OldVersion>(Convert.FromHexString(fixtures["version-new"]))!;
if (previous.Value != 42 || previous.Text != "hello" || previous.Removed != 0)
    throw new Exception("New-to-old version-tolerant schema failed");
NewObject? existing = new() { Text = "existing" };
MemoryPackSerializer.Deserialize(Convert.FromHexString(fixtures["object-old"]), ref existing);
if (existing!.Value != 42 || existing.Text != "existing")
    throw new Exception("Ordinary overwrite must preserve absent members");
NewVersion? existingVersion = new() { Added = 100 };
MemoryPackSerializer.Deserialize(Convert.FromHexString(fixtures["version-old"]), ref existingVersion);
if (existingVersion!.Added != 100)
    throw new Exception("Version overwrite must preserve absent members");
OldVersion? existingOldVersion = new() { Removed = 100 };
MemoryPackSerializer.Deserialize(Convert.FromHexString(fixtures["version-new"]), ref existingOldVersion);
if (existingOldVersion!.Removed != 100)
    throw new Exception("Version overwrite must preserve deleted members");

var json = JsonSerializer.Serialize(fixtures, new JsonSerializerOptions { WriteIndented = true }).Replace("\r\n", "\n") + "\n";
if (args.Length != 2 || (args[0] != "--write" && args[0] != "--check"))
    throw new ArgumentException("Usage: --write PATH | --check PATH");
if (args[0] == "--write")
    File.WriteAllText(args[1], json);
else if (File.ReadAllText(args[1]).Replace("\r\n", "\n") != json)
    throw new Exception("Checked-in fixture bytes differ from MemoryPack 1.21.4 output");
Console.WriteLine($"Verified {fixtures.Count} C# MemoryPack 1.21.4 fixtures.");

[MemoryPackable]
[MemoryPackUnion(0, typeof(NumberMessage))]
[MemoryPackUnion(249, typeof(BoundaryMessage))]
[MemoryPackUnion(250, typeof(TextMessage))]
[MemoryPackUnion(65535, typeof(MaxMessage))]
public partial interface IMessage { }

[MemoryPackable]
public partial class NumberMessage : IMessage { public int Value { get; set; } }
[MemoryPackable]
public partial class BoundaryMessage : IMessage { public int Value { get; set; } }
[MemoryPackable]
public partial class TextMessage : IMessage { public string? Value { get; set; } }
[MemoryPackable]
public partial class MaxMessage : IMessage { public ushort Value { get; set; } }
[MemoryPackable]
public partial class Envelope { public IMessage? Message { get; set; } }
[MemoryPackable]
public partial class OldObject { public int Value { get; set; } }
[MemoryPackable]
public partial class NewObject { public int Value { get; set; } public string? Text { get; set; } }
[MemoryPackable]
public partial class NullableHolder { public int? Number { get; set; } public string? Text { get; set; } }

[MemoryPackable(GenerateType.VersionTolerant)]
public partial class OldVersion
{
    [MemoryPackOrder(0)] public int Value { get; set; }
    [MemoryPackOrder(1)] public long Removed { get; set; }
    [MemoryPackOrder(2)] public string? Text { get; set; }
}
[MemoryPackable(GenerateType.VersionTolerant)]
public partial class NewVersion
{
    [MemoryPackOrder(0)] public int Value { get; set; }
    [MemoryPackOrder(2)] public string? Text { get; set; }
    [MemoryPackOrder(3)] public short Added { get; set; }
}
[MemoryPackable(GenerateType.VersionTolerant)]
public partial class LongVersion
{
    [MemoryPackOrder(0)] public string? Text { get; set; }
}
