Summary: Simple Network Backup Utility
Name: pukcab
Version: 1.1
Release: 1
Source: http://www.ezix.org/software/files/%{name}-%{version}-Linux-%{_arch}.zip
URL: http://ezix.org/project/wiki/Pukcab
License: GPL
Group: Applications/System
BuildRoot: %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)
Requires: openssh-clients

%description
Pukcab is a lightweight, single-binary backup system that stores de-duplicated, compressed and incremental backups on a remote server using just an SSH connection.

%prep
%setup -q -c %{name}-%{version}

%build

%install
%{__rm} -rf "%{buildroot}"

%{__install} -D pukcab %{buildroot}%{_bindir}/pukcab

%clean
%{__rm} -rf %{buildroot}

%files
%defattr(-,root,root, 0555)
%doc README.md
%{_bindir}/*

%changelog
* Wed Feb 18 2015 Lyonel Vincent <lyonel@ezix.org> 1.1-1
- RPM packaging
