
# PikaMgr #
PikaMgr is based on Codis3.2.2-Dashboard and Codis3.2.2-FE([https://github.com/CodisLabs/codis/tree/3.2.2](https://github.com/CodisLabs/codis/tree/3.2.2)).

# Feature #

* Removed the original proxy and slot related management functions of codis
* The files index.html and dashboard-fe.js in the front-end assets of codis-fe have been partially modified to support related functions
* Support for the pika info format can be displayed in the native info format and display normal Memory, DBSize, and Keys information
* Management of lvs+haproxy agent scheme
* Management of related configuration template files (Note: Currently, the platform does not support CUD operations for template files, only supports viewing)
* Use the package in the golang base library whenever possible without affecting code functionality and code cleanliness

# Documents #
[wiki](https://github.com/pourer/pikamgr/wiki)

# Dependencies #
> When updating dependencies with `glide update`, need to use `--strip-vendor` option. Otherwise there will be type conflicts.

      glide update --strip-vendor
