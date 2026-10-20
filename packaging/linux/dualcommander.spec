Name: dualcommander
Version: %{package_version}
Release: %{package_release}
Summary: Two-pane desktop file manager built with Go and Fyne
License: MIT
URL: https://dualcommander.github.io/
BuildArch: %{package_arch}
Requires: glibc >= %{glibc_version}, glib2, xdg-utils
# ponytail: package existing Fyne build; change baseline before supporting older glibc.
%global debug_package %{nil}
%global __os_install_post %{nil}
%description
Two-pane desktop file manager built with Go and Fyne.
%install
mkdir -p %{buildroot}
cp -a "%{payload_root}/." %{buildroot}/
%files
/usr/bin/dualcommander
/usr/share/applications/*.desktop
/usr/share/pixmaps/*.png
%license /usr/share/licenses/dualcommander/LICENSE
