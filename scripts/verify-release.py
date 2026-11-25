#!/usr/bin/env python3
"""Verifies published artifacts and APT metadata on Ubuntu without changing system configuration."""

import argparse
import getpass
import gzip
import hashlib
import json
from pathlib import Path
import subprocess
import urllib.request


def download(url, destination):
    """Downloads a public URL to the supplied path and returns its bytes.

    Creates parent directories. Network and filesystem errors stop verification.
    """
    request = urllib.request.Request(url, headers={"User-Agent": "hddmount-release-check"})

    with urllib.request.urlopen(request, timeout=60) as response:
        content = response.read()

    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_bytes(content)

    return content


def run(*arguments, cwd=None, include_stderr=False):
    """Runs arguments without a shell and returns stdout.

    If include_stderr is true, appends diagnostic output to stdout.
    A nonzero exit raises an exception that includes captured diagnostic output.
    """
    result = subprocess.run(arguments, cwd=cwd, text=True, capture_output=True)

    if result.returncode:
        raise RuntimeError(f"{arguments[0]} failed:\n{result.stdout}\n{result.stderr}")

    return result.stdout + (result.stderr if include_stderr else "")


def fields(paragraph):
    """Returns Debian metadata fields from one paragraph without continuation lines."""
    return dict(line.split(": ", 1) for line in paragraph.splitlines() if ": " in line and not line.startswith(" "))


def verify(tag, previous_tag, destination):
    """Verifies release assets, signatures, index hashes, APT candidates, and the amd64 payload.

    Accepts public version tags and a fresh workspace output directory.
    Writes downloaded artifacts and isolated tool state only beneath destination.
    Raises on any mismatch. It does not mount disks or modify system APT settings.
    """
    destination.mkdir(parents=True, exist_ok=False)
    repository = "LeeAn0121/hddmount-tui"
    base = "https://leean0121.github.io/hddmount-tui"
    version = tag.removeprefix("v")
    api = f"https://api.github.com/repos/{repository}/releases/tags/{tag}"
    release = json.loads(download(api, destination / "release.json"))
    assets = {asset["name"]: asset["browser_download_url"] for asset in release["assets"]}
    expected = {"checksums.txt"}

    for architecture in ("amd64", "arm64"):
        expected.add(f"hddmount-tui_{version}_{architecture}.deb")
        expected.add(f"hddmount-tui_{version}_linux_{architecture}.tar.gz")

    assert set(assets) == expected, assets.keys()
    assert not release["draft"] and not release["prerelease"]

    for name, url in assets.items():
        download(url, destination / name)

    checksums = {name: digest for digest, name in
                 (line.split() for line in (destination / "checksums.txt").read_text().splitlines())}

    for name in expected - {"checksums.txt"}:
        assert hashlib.sha256((destination / name).read_bytes()).hexdigest() == checksums[name], name

    print("Release: both architectures, .deb, tar.gz, SHA256 passed")
    homepage = download(base + "/", destination / "index.html")
    assert tag.encode() in homepage
    assert b"link-uploads" in homepage

    for name in ("pubkey.gpg", "dists/stable/Release", "dists/stable/Release.gpg", "dists/stable/InRelease"):
        download(base + "/" + name, destination / name)

    keyhome = destination / "gnupg"
    keyhome.mkdir(mode=0o700)
    gpg = ("gpg", "--homedir", str(keyhome), "--batch")
    run(*gpg, "--import", str(destination / "pubkey.gpg"))
    run(*gpg, "--verify", str(destination / "dists/stable/Release.gpg"), str(destination / "dists/stable/Release"))
    run(*gpg, "--output", str(destination / "verified-release"), "--decrypt", str(destination / "dists/stable/InRelease"))
    assert (destination / "verified-release").read_bytes() == (destination / "dists/stable/Release").read_bytes()
    keyring = destination / "keyring.gpg"
    run(*gpg, "--output", str(keyring), "--dearmor", str(destination / "pubkey.gpg"))
    release_text = (destination / "dists/stable/Release").read_text()
    release_hashes = {}
    in_sha256 = False

    for line in release_text.splitlines():
        if line == "SHA256:":
            in_sha256 = True
            continue

        if in_sha256 and line.startswith(" "):
            digest, size, name = line.split()
            release_hashes[name] = (digest, int(size))
        elif in_sha256:
            break

    for architecture in ("amd64", "arm64"):
        relative = f"main/binary-{architecture}/Packages"
        packages = download(base + "/dists/stable/" + relative, destination / relative)
        compressed = download(base + "/dists/stable/" + relative + ".gz", destination / (relative + ".gz"))
        assert gzip.decompress(compressed) == packages

        for name, content in ((relative, packages), (relative + ".gz", compressed)):
            assert release_hashes[name] == (hashlib.sha256(content).hexdigest(), len(content)), name

        entries = [fields(paragraph) for paragraph in packages.decode().strip().split("\n\n")]
        package = next(item for item in entries if item.get("Package") == "hddmount-tui")
        assert package["Version"] == version and package["Architecture"] == architecture
        name = f"hddmount-tui_{version}_{architecture}.deb"
        content = download(base + "/" + package["Filename"], destination / "pool" / name)
        assert package["SHA256"] == checksums[name] == hashlib.sha256(content).hexdigest()
        assert int(package["Size"]) == len(content)
        assert run("dpkg-deb", "-f", str(destination / name), "Version").strip() == version
        assert run("dpkg-deb", "-f", str(destination / name), "Architecture").strip() == architecture

        apt = destination / f"apt-{architecture}"

        for directory in ("state/lists/partial", "cache/archives/partial", "sourceparts", "downloads", "log"):
            (apt / directory).mkdir(parents=True)

        (apt / "state/status").touch()
        (apt / "sources.list").write_text(f"deb [arch={architecture} signed-by={keyring}] {base} stable main\n")
        options = ["-o", f"Dir::State={apt / 'state'}", "-o", f"Dir::State::status={apt / 'state/status'}",
                   "-o", f"Dir::Cache={apt / 'cache'}", "-o", f"Dir::Log={apt / 'log'}",
                   "-o", f"Dir::Etc::sourcelist={apt / 'sources.list'}", "-o", f"Dir::Etc::sourceparts={apt / 'sourceparts'}",
                   "-o", f"APT::Architecture={architecture}", "-o", f"APT::Architectures::={architecture}",
                   "-o", f"APT::Sandbox::User={getpass.getuser()}", "-o", "APT::Get::List-Cleanup=0"]
        (apt / "update.log").write_text(run("apt-get", *options, "update", "--error-on=any"))
        policy = run("apt-cache", *options, "policy", "hddmount-tui")
        (apt / "policy.log").write_text(policy)
        assert f"Candidate: {version}" in policy, policy
        run("apt-get", *options, "download", f"hddmount-tui:{architecture}={version}", cwd=apt / "downloads")
        downloaded = next((apt / "downloads").glob("*.deb"))
        assert hashlib.sha256(downloaded.read_bytes()).hexdigest() == checksums[name]
        print(f"APT {architecture}: signature, index hashes, candidate {version}, package download passed")

    root = destination / "installed-root"
    (root / "var/lib/dpkg").mkdir(parents=True)
    (root / "var/lib/dpkg/status").touch()
    old_version = previous_tag.removeprefix("v")
    old_name = f"hddmount-tui_{old_version}_amd64.deb"
    download(f"https://github.com/{repository}/releases/download/{previous_tag}/{old_name}", destination / old_name)

    dependencies = run("dpkg-deb", "-f", str(destination / f"hddmount-tui_{version}_amd64.deb"), "Depends").strip()

    for install_version in (old_version, version):
        name = destination / f"hddmount-tui_{install_version}_amd64.deb"

        if dependencies:
            run("dpkg-deb", "--extract", str(name), str(root))
            continue

        output = run("dpkg", "--force-not-root", f"--root={root}", f"--log={destination / 'dpkg.log'}", "--install", str(name))
        (destination / f"install-{install_version}.log").write_text(output)
        installed = run("dpkg-query", f"--admindir={root / 'var/lib/dpkg'}", "-W", "-f=${Version}", "hddmount-tui")
        assert installed == install_version, installed

    binary = root / "usr/bin/hddmount"
    assert "link-uploads" in run(str(binary), "--help", include_stderr=True)
    assert binary.read_bytes() == run_archive_binary(destination, version)
    if dependencies:
        print(f"Ubuntu amd64 payload extraction {old_version} -> {version}, CLI help, archive binary match passed")
        print("Dependency installation and APT upgrades are tested in the Ubuntu compatibility workflow.")
    else:
        print(f"Ubuntu amd64 isolated dpkg install {old_version} -> {version}, CLI help, archive binary match passed")
    print("Pages: version and link-uploads passed. No disk mount or live upload-path mutation was performed.")


def run_archive_binary(destination, version):
    """Returns the amd64 binary from the verified archive without extracting filesystem paths."""
    import tarfile

    with tarfile.open(destination / f"hddmount-tui_{version}_linux_amd64.tar.gz") as archive:
        return archive.extractfile("hddmount").read()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("tag")
    parser.add_argument("--previous", default="v0.3.6")
    parser.add_argument("--output", required=True, type=Path)
    arguments = parser.parse_args()
    verify(arguments.tag, arguments.previous, arguments.output.resolve())
