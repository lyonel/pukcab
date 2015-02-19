Summary: Simple Network Backup Utility
Name: pukcab
Version: %{VERSION}
Release: 1
Source: http://www.ezix.org/software/files/%{name}-%{version}-Linux-%{_arch}.zip
URL: http://ezix.org/project/wiki/Pukcab
License: GPL
Group: Applications/System

%description
Pukcab is a lightweight, single-binary backup system that stores de-duplicated, compressed and incremental backups on a remote server using just an SSH connection.

%package client
BuildArch: noarch
Summary: Simple Network Backup Utility (client)
Requires: openssh-clients
Requires: %{name} >= %{version}

%description client
Pukcab is a lightweight, single-binary backup system that stores de-duplicated, compressed and incremental backups on a remote server using just an SSH connection.

This package ensures a system can act as a pukcab client.

%package server
BuildArch: noarch
Summary: Simple Network Backup Utility (server)
Requires: openssh-server
Requires: %{name} >= %{version}

%description server
Pukcab is a lightweight, single-binary backup system that stores de-duplicated, compressed and incremental backups on a remote server using just an SSH connection.

This package ensures a system can act as a pukcab server.

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
%config %ghost %{_sysconfdir}/%{name}.conf
%{_bindir}/*

%files client

%files server

%changelog
* Wed Feb 18 2015 Lyonel Vincent <lyonel@ezix.org> 1.1-1
- RPM packaging
