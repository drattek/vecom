#!/usr/bin/env python3
"""Añade volúmenes de Azure Files o cambia los args de un contenedor de una Container App.

`az containerapp create/update` no tiene parámetros para volúmenes:
hay que pasar la plantilla por --yaml. Este helper lee el JSON de
`az containerapp show`, aplica el cambio y escribe un YAML (JSON es YAML válido)
con SOLO properties.template, para no tocar secretos ni ingress.

Idempotente: si el volumen ya existe, se reemplaza.

    patch_template.py volume  show.json out.json --volume redis-data --storage redis-data \
        --container vecom-redis --mount-path /data [--mount-options "uid=999,gid=999"]
    patch_template.py container show.json out.json --container vecom-redis \
        --args redis-server --appendonly yes
"""
import argparse
import json


def upsert(items, item, key="name"):
    items[:] = [i for i in items if i.get(key) != item[key]] + [item]


def container(template, name):
    for c in template["containers"]:
        if c["name"] == name:
            return c
    raise SystemExit(f"no existe el contenedor {name}")


def main():
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)

    vol = sub.add_parser("volume")
    vol.add_argument("--volume", required=True)
    vol.add_argument("--storage", required=True)
    vol.add_argument("--container", required=True)
    vol.add_argument("--mount-path", required=True)
    vol.add_argument("--mount-options")

    cont = sub.add_parser("container")
    cont.add_argument("--container", required=True)
    cont.add_argument("--args", nargs=argparse.REMAINDER, default=[])

    for p in (vol, cont):
        p.add_argument("show_json")
        p.add_argument("out_json")
    args = parser.parse_args()

    app = json.load(open(args.show_json))
    template = app["properties"]["template"]

    if args.cmd == "volume":
        volume = {"name": args.volume, "storageType": "AzureFile", "storageName": args.storage}
        if args.mount_options:
            volume["mountOptions"] = args.mount_options
        template.setdefault("volumes", [])
        template["volumes"] = template["volumes"] or []
        upsert(template["volumes"], volume)
        c = container(template, args.container)
        c["volumeMounts"] = c.get("volumeMounts") or []
        upsert(c["volumeMounts"], {"volumeName": args.volume, "mountPath": args.mount_path}, key="volumeName")

    elif args.cmd == "container":
        container(template, args.container)["args"] = args.args

    json.dump({"properties": {"template": template}}, open(args.out_json, "w"), indent=2)


if __name__ == "__main__":
    main()
