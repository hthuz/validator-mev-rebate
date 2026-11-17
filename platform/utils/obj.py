

def obj_to_dict(obj):
    result = {}
    for cls in obj.__class__.mro():
        if hasattr(cls, '__dict__') and '__init__' in cls.__dict__:
            instance_vars = vars(obj)

            if "stakes" in instance_vars.keys():
                instance_vars.pop("stakes")
            if "smap" in instance_vars.keys():
                instance_vars.pop("smap")
            if "vmap" in instance_vars.keys():
                instance_vars.pop("vmap")

            result.update(instance_vars)
    return result