## Limitations

### sanitized app names

`rkt app` could fail if it runs with an app name that contains uppercase
letters, because the [appc spec][acname] does not allow uppercase letters.
It requires an ACName of lowercase letters plus `"-"`.
So even if a client passes to rktlet an input name with uppercase letters
like `FooBar`, rktlet will sanitize the string to lowercase letters like
`foobar`, e.g.:

```shell
$ sudo rkt app add 01234567-89ab-cdef-0123-456789abcdef docker://busybox:latest \
  --name=foobar
```

However, the approach has a limitation. If two apps are added to a pod,
one with a name `FooBar`, another with `foobar`, it will fail because
both will be sanitized to the same name `foobar`.

[acname]: https://github.com/appc/spec/blob/v0.8.11/schema/types/acname.go#L38..L45
