{
  inputs,
  system,
  ...
}:
let
  data = "/data/flash";
  port = "8765";
in
{
  systemd.tmpfiles.rules = [
    "d ${data} 0755 root root - -"
  ];

  virtualisation.oci-containers.containers = {
    flash = {
      image = "flash:latest";
      imageFile = inputs.flash.packages.${system}.docker;
      ports = [ "127.0.0.1:${port}:8765" ];
      volumes = [
        "${data}:/data"
      ];
      environment = {
        FLASH_SERVE_HOST  = "0.0.0.0";
        FLASH_SERVE_PORT  = "8765";
        FLASH_SERVE_TOKEN = "change-me";
        FLASH_SERVE_DATA  = "/data";
      };
    };
  };

  services.nginx.virtualHosts."flash.chambaz.xyz" = {
    enableACME = true;
    forceSSL = true;

    locations."/" = {
      proxyPass = "http://127.0.0.1:${port}";
      extraConfig = ''
        proxy_http_version 1.1;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host  $host;
      '';
    };
  };
}
