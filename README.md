This is a Rational Team Concert automated project area creation tool.
It is useful for testing and development purposes.

    projectcreator -repo=https://myserver.com:9443/ccm -name=NewProjectArea -user=AdminUser -password=AdminPassword -processid=scrum2.process.ibm.com -members="AdminUser=Team Member,OtherUser=Team Member"

You can get a listing of the process templates (for the -processid parameter)
from the server like this:  
  
    projectcreator -repo=https://myserver.com:9443/ccm -user=AdminUser -password=AdminPassword -templates

Each time you invoke the command it will automatically deploy all of the process templates
included with your server install. If they are already deployed, either manually or using
the tool you can add the "-nodeploy" parameter to skip that step and improve performance.

